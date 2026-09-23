using System;
using System.Diagnostics;
using System.IO;
using System.Reflection;
using System.Text;
using System.Threading.Tasks;
using EnvDTE;
using Microsoft.VisualStudio;
using Microsoft.VisualStudio.Shell.Interop;
using TCatSysManagerLib;

namespace format
{
    [Command("{030fb4a7-a21d-4b36-a28d-ffc41b84fbfc}", 0x0100)]
    internal sealed class Format : BaseCommand<Format>
    {
        private static readonly string[] SupportedExtensions =
        {
            ".st", ".iecst", ".tcst",
            ".tcpou", ".tcgvl", ".tcdut", ".tcio",
        };

        protected override async Task ExecuteAsync(OleMenuCmdEventArgs e)
        {
            await ThreadHelper.JoinableTaskFactory.SwitchToMainThreadAsync();

            string executable = FindExecutable();
            if (executable is null)
            {
                Log.Error("stformat.exe not found");
                await VS.MessageBox.ShowErrorAsync(
                    "Format",
                    "Could not find stformat.exe. Install it (for example with " +
                    "'winget install --id Ysmilda.stformat -e') or place stformat.exe next to this extension.");
                return;
            }

            Log.Info($"Using stformat: {executable}");

            // TwinCAT PLC objects (POUs and their methods/actions, GVLs, DUTs,
            // interfaces) expose an automation model. The active document
            // moniker of a method carries a "@method" suffix, so the automation
            // object is used instead of the file path. Updating the object
            // keeps the change inside the environment, so the TwinCAT editor
            // does not report the file as modified externally.
            DTE dte = GetDTE();
            if (TryGetPlcObject(dte, out ITcPlcDeclaration declaration, out ITcPlcImplementation implementation))
            {
                Log.Info("Formatting TwinCAT PLC object in place");
                await GuardAsync(() => FormatPlcObjectAsync(executable, dte, declaration, implementation));
                return;
            }

            DocumentView view = await VS.Documents.GetActiveDocumentViewAsync();

            await ThreadHelper.JoinableTaskFactory.SwitchToMainThreadAsync();
            string framePath = GetActiveDocumentPath();

            string filePath = PickSupportedDocument(view?.FilePath, framePath);
            if (filePath is null)
            {
                Log.Warning("No supported active document found to format");
                string detail = string.Empty;
                if (view?.FilePath is not null)
                {
                    detail += Environment.NewLine + "Text view: " + view.FilePath;
                }
                if (framePath is not null)
                {
                    detail += Environment.NewLine + "Active frame: " + framePath;
                }

                await VS.MessageBox.ShowWarningAsync(
                    "Format",
                    "There is no supported active document to format." + detail);
                return;
            }

            // Persist any pending edits so the file on disk matches the editor.
            // The text view has no document for custom editors, so fall back to
            // the shell's Save command for those.
            if (view?.Document is not null && IsSamePath(view.FilePath, filePath))
            {
                view.Document.Save();
            }
            else
            {
                await VS.Commands.ExecuteAsync("File.SaveSelectedItems");
            }

            int exitCode;
            string output;
            try
            {
                (exitCode, output) = await RunAsync(executable, filePath);
            }
            catch (Exception ex)
            {
                Log.Error($"Failed to run stformat for {filePath}: {ex}");
                await VS.MessageBox.ShowErrorAsync("Format", $"Failed to run stformat: {ex.Message}");
                return;
            }

            if (exitCode != 0)
            {
                string message = string.IsNullOrEmpty(output)
                    ? "stformat exited with a non-zero exit code."
                    : output;
                Log.Error($"stformat exited with code {exitCode} for {filePath}");
                await VS.MessageBox.ShowErrorAsync("Format", $"stformat failed:{Environment.NewLine}{message}");
                return;
            }

            Log.Info($"stformat completed for {filePath}");

            // Reflect the formatted content in the open editor. Custom editors
            // (for example TwinCAT) don't expose a text document view for the
            // project file, so reload those by reopening the document.
            if (view?.Document is not null && IsSamePath(view.FilePath, filePath))
            {
                view.Document.Reload();
            }
            else
            {
                await ReloadDocumentAsync(filePath);
            }
        }

        // Formats a TwinCAT PLC object through the automation interface so the
        // change is applied to the open document instead of the file on disk.
        private static async Task FormatPlcObjectAsync(
            string executable,
            DTE dte,
            ITcPlcDeclaration declaration,
            ITcPlcImplementation implementation)
        {
            // Persist pending edits first so the automation model is current.
            await ThreadHelper.JoinableTaskFactory.SwitchToMainThreadAsync();
            TrySave(dte);

            if (declaration is not null)
            {
                string original = declaration.DeclarationText;
                string formatted = await FormatTextAsync(executable, original);
                if (formatted is null)
                {
                    return;
                }

                formatted = WrapText(formatted);
                if (Differs(formatted, original))
                {
                    Log.Info("PLC object declaration reformatted");
                    declaration.DeclarationText = formatted;
                }
            }

            // Only Structured Text implementations can be formatted; other
            // languages (FBD, LD, ...) are left untouched.
            if (implementation is not null && IsStructuredText(implementation))
            {
                string original = implementation.ImplementationText;
                string formatted = await FormatTextAsync(executable, original);
                if (formatted is null)
                {
                    return;
                }

                formatted = WrapText(formatted);
                if (Differs(formatted, original))
                {
                    Log.Info("PLC object implementation reformatted");
                    implementation.ImplementationText = formatted;
                }
            }

            await ThreadHelper.JoinableTaskFactory.SwitchToMainThreadAsync();
            TrySave(dte);
        }

        // Guards against re-entrancy: formatting saves the document, which
        // raises the save event again.
        private static bool _formatting;

        // Subscribes the format-on-save hook. Must be called on the UI thread
        // during package initialization.
        internal static void RegisterOnSave()
        {
            ThreadHelper.ThrowIfNotOnUIThread();
            VS.Events.DocumentEvents.Saved += OnDocumentSaved;
            Log.Info("Document save handler subscribed");
        }

        private static readonly ToolkitThreadHelper _toolkitThreadHelper = ToolkitThreadHelper.Create();

        private static void OnDocumentSaved(string moniker)
        {
            Log.Info($"Document saved: {moniker}");
            _toolkitThreadHelper.JoinableTaskFactory.RunAsync(async () =>
            {
                try
                {
                    await _toolkitThreadHelper.JoinableTaskFactory.SwitchToMainThreadAsync(_toolkitThreadHelper.DisposalToken);
                    await GuardAsync(() => FormatSavedDocumentAsync(moniker));
                    Log.Info($"Format-on-save finished: {moniker}");
                }
                catch (Exception ex)
                {
                    Log.Error($"Format-on-save failed for {moniker}: {ex}");
                }
            }).FireAndForget();
        }

        // Formats a document that was just saved. TwinCAT PLC objects are
        // formatted through the automation interface; other supported files
        // are formatted on disk and reloaded in the editor.
        private static async Task FormatSavedDocumentAsync(string moniker)
        {
            if (string.IsNullOrEmpty(moniker))
            {
                Log.Warning("Format-on-save skipped: empty moniker");
                return;
            }

            await ThreadHelper.JoinableTaskFactory.SwitchToMainThreadAsync();

            string executable = FindExecutable();
            if (executable is null)
            {
                Log.Warning("Format-on-save skipped: stformat.exe not found");
                return;
            }

            DTE dte = GetDTE();
            if (dte is null)
            {
                Log.Warning("Format-on-save skipped: DTE unavailable");
                return;
            }

            object automation = GetAutomationObject(dte, moniker);
            ITcPlcDeclaration declaration = automation as ITcPlcDeclaration;
            ITcPlcImplementation implementation = automation as ITcPlcImplementation;
            if (declaration is not null || implementation is not null)
            {
                Log.Info($"Format-on-save: TwinCAT PLC object {moniker}");
                await FormatPlcObjectAsync(executable, dte, declaration, implementation);
                return;
            }

            if (!IsSupported(moniker) || !File.Exists(moniker))
            {
                Log.Info($"Format-on-save skipped: {moniker}");
                return;
            }

            await FormatFileAsync(executable, moniker);
        }

        // Formats a supported file on disk and refreshes the open text document
        // when the content changed. The file is only reloaded when stformat
        // actually modified it, so a no-op save does not disturb the editor.
        private static async Task FormatFileAsync(string executable, string filePath)
        {
            string before = File.ReadAllText(filePath);

            (int exitCode, _) = await RunAsync(executable, filePath);
            if (exitCode != 0)
            {
                Log.Error($"stformat exited with code {exitCode} for {filePath}");
                return;
            }

            string after = File.ReadAllText(filePath);
            if (before == after)
            {
                Log.Info($"No formatting changes for {filePath}");
                return;
            }

            Log.Info($"File formatted on disk: {filePath}");
            DocumentView view = await VS.Documents.GetActiveDocumentViewAsync();
            await ThreadHelper.JoinableTaskFactory.SwitchToMainThreadAsync();
            if (view?.Document is not null && IsSamePath(view.FilePath, filePath))
            {
                view.Document.Reload();
            }
        }

        // Finds the automation object (if any) of an open document by its
        // moniker. TwinCAT method documents carry a "@method" suffix in the
        // moniker, which is why the document is matched instead of the file.
        private static object GetAutomationObject(DTE dte, string moniker)
        {
            ThreadHelper.ThrowIfNotOnUIThread();

            try
            {
                foreach (Document document in dte.Documents)
                {
                    if (!IsSamePath(document.FullName, moniker))
                    {
                        continue;
                    }

                    try
                    {
                        return document.ProjectItem?.Object;
                    }
                    catch (Exception)
                    {
                        return null;
                    }
                }
            }
            catch (Exception)
            {
                // Fall through to null.
            }

            return null;
        }

        private static async Task GuardAsync(Func<Task> action)
        {
            if (_formatting)
            {
                return;
            }

            _formatting = true;
            try
            {
                await action();
            }
            finally
            {
                _formatting = false;
            }
        }

        // Formats a piece of ST text by writing it to a temporary file and
        // running stformat on it. Returns null and reports the error when
        // stformat fails.
        private static async Task<string> FormatTextAsync(string executable, string text)
        {
            if (string.IsNullOrEmpty(text))
            {
                return text;
            }

            string temporary = Path.Combine(Path.GetTempPath(), Path.GetRandomFileName() + ".st");
            try
            {
                File.WriteAllText(temporary, text, new UTF8Encoding(false));

                int exitCode;
                string output;
                try
                {
                    (exitCode, output) = await RunAsync(executable, temporary);
                }
                catch (Exception ex)
                {
                    Log.Error($"Failed to run stformat on temporary file: {ex}");
                    await VS.MessageBox.ShowErrorAsync("Format", $"Failed to run stformat: {ex.Message}");
                    return null;
                }

                if (exitCode != 0)
                {
                    string message = string.IsNullOrEmpty(output)
                        ? "stformat exited with a non-zero exit code."
                        : output;
                    Log.Error($"stformat exited with code {exitCode}");
                    await VS.MessageBox.ShowErrorAsync("Format", $"stformat failed:{Environment.NewLine}{message}");
                    return null;
                }

                return File.ReadAllText(temporary, Encoding.UTF8);
            }
            finally
            {
                try
                {
                    File.Delete(temporary);
                }
                catch (IOException)
                {
                    // Ignore leftover temporary files.
                }
            }
        }

        // Refreshes a document hosted by a custom editor by closing and
        // reopening its window frame, keeping the file open.
        private static async Task ReloadDocumentAsync(string filePath)
        {
            WindowFrame frame = await VS.Windows.FindDocumentWindowAsync(filePath);
            if (frame is not null)
            {
                await frame.CloseFrameAsync(FrameCloseOption.NoSave);
            }

            await VS.Documents.OpenViaProjectAsync(filePath);
        }

        private static DTE GetDTE()
        {
            ThreadHelper.ThrowIfNotOnUIThread();
            return (DTE)Microsoft.VisualStudio.Shell.Package.GetGlobalService(typeof(DTE));
        }

        // Tries to obtain the TwinCAT automation object for the active
        // document. Returns true when the active document is a PLC object
        // (POU, GVL, DUT or interface) that can be formatted in place.
        private static bool TryGetPlcObject(
            DTE dte,
            out ITcPlcDeclaration declaration,
            out ITcPlcImplementation implementation)
        {
            ThreadHelper.ThrowIfNotOnUIThread();

            declaration = null;
            implementation = null;

            if (dte is null)
            {
                return false;
            }

            object automation;
            try
            {
                automation = dte.ActiveWindow?.ProjectItem?.Object;
            }
            catch (Exception)
            {
                return false;
            }

            if (automation is null)
            {
                return false;
            }

            declaration = automation as ITcPlcDeclaration;
            implementation = automation as ITcPlcImplementation;
            return declaration is not null || implementation is not null;
        }

        private static bool IsStructuredText(ITcPlcImplementation implementation)
        {
            try
            {
                return implementation.Language == IECLANGUAGETYPES.IECLANGUAGE_ST;
            }
            catch (Exception)
            {
                return false;
            }
        }

        private static void TrySave(DTE dte)
        {
            ThreadHelper.ThrowIfNotOnUIThread();
            try
            {
                dte.ActiveDocument?.Save("");
            }
            catch (Exception)
            {
                // The document may not be saveable; the change stays in memory.
            }
        }

        // Compares two pieces of ST text ignoring line ending style and
        // trailing newlines.
        private static bool Differs(string left, string right)
        {
            return Normalize(left) != Normalize(right);
        }

        // The file based formatter stores the declaration and implementation
        // content with a single leading and trailing line break inside the
        // CDATA section. Reproduce that so the automation path produces the
        // same output.
        private static string WrapText(string formatted)
        {
            return "\n" + formatted.Trim('\r', '\n') + "\n";
        }

        private static string Normalize(string text)
        {
            return text?.Replace("\r\n", "\n").TrimEnd('\n');
        }

        // Picks the first candidate that is a supported file. Custom editors may
        // present a temporary .txt file as their active view, so the pair is
        // checked rather than trusting the text view blindly. When neither
        // candidate is supported, a running document with the same base name
        // (for example MAIN.TcPOU for MAIN.txt) is used as a last resort.
        private static string PickSupportedDocument(string textViewPath, string framePath)
        {
            ThreadHelper.ThrowIfNotOnUIThread();

            if (IsSupported(textViewPath))
            {
                return textViewPath;
            }

            if (IsSupported(framePath))
            {
                return framePath;
            }

            string baseName = Path.GetFileNameWithoutExtension(framePath ?? textViewPath);
            return baseName is null ? null : FindRunningDocument(baseName);
        }

        private static bool IsSupported(string path)
        {
            if (string.IsNullOrEmpty(path))
            {
                return false;
            }

            string extension = Path.GetExtension(path).ToLowerInvariant();
            return Array.IndexOf(SupportedExtensions, extension) >= 0;
        }

        private static bool IsSamePath(string left, string right)
        {
            return string.Equals(left, right, StringComparison.OrdinalIgnoreCase);
        }

        // Finds an open document with a supported extension whose file name
        // matches the given base name.
        private static string FindRunningDocument(string baseName)
        {
            ThreadHelper.ThrowIfNotOnUIThread();

            var rdt = (IVsRunningDocumentTable)Microsoft.VisualStudio.Shell.Package.GetGlobalService(typeof(SVsRunningDocumentTable));
            if (rdt is null || rdt.GetRunningDocumentsEnum(out IEnumRunningDocuments documents) != VSConstants.S_OK || documents is null)
            {
                return null;
            }

            string match = null;
            var cookies = new uint[1];
            while (documents.Next(1, cookies, out uint fetched) == VSConstants.S_OK && fetched == 1)
            {
                if (rdt.GetDocumentInfo(cookies[0], out _, out _, out _, out string moniker, out _, out _, out _) != VSConstants.S_OK)
                {
                    continue;
                }

                if (!IsSupported(moniker) ||
                    !string.Equals(Path.GetFileNameWithoutExtension(moniker), baseName, StringComparison.OrdinalIgnoreCase))
                {
                    continue;
                }

                if (match is not null)
                {
                    // Ambiguous, don't guess.
                    return null;
                }

                match = moniker;
            }

            return match;
        }

        // Gets the file path of the active document even when it is hosted by a
        // custom editor (for example TwinCAT) that has no text document view.
        private static string GetActiveDocumentPath()
        {
            ThreadHelper.ThrowIfNotOnUIThread();

            var selection = (IVsMonitorSelection)Microsoft.VisualStudio.Shell.Package.GetGlobalService(typeof(SVsShellMonitorSelection));
            if (selection is null)
            {
                return null;
            }

            if (selection.GetCurrentElementValue((uint)VSConstants.VSSELELEMID.SEID_DocumentFrame, out object value) != VSConstants.S_OK)
            {
                return null;
            }

            if (value is not IVsWindowFrame frame)
            {
                return null;
            }

            if (frame.GetProperty((int)__VSFPROPID.VSFPROPID_pszMkDocument, out object path) != VSConstants.S_OK)
            {
                return null;
            }

            return path as string;
        }

        private static string FindExecutable()
        {
            string assemblyDirectory = Path.GetDirectoryName(Assembly.GetExecutingAssembly().Location) ?? string.Empty;

            string bundled = Path.Combine(assemblyDirectory, "stformat.exe");
            if (File.Exists(bundled))
            {
                return bundled;
            }

            string bundledResources = Path.Combine(assemblyDirectory, "Resources", "stformat.exe");
            if (File.Exists(bundledResources))
            {
                return bundledResources;
            }

            string pathVariable = Environment.GetEnvironmentVariable("PATH");
            if (pathVariable is null)
            {
                return null;
            }

            foreach (string directory in pathVariable.Split(Path.PathSeparator))
            {
                if (string.IsNullOrWhiteSpace(directory))
                {
                    continue;
                }

                string candidate = Path.Combine(directory.Trim().Trim('"'), "stformat.exe");
                if (File.Exists(candidate))
                {
                    return candidate;
                }
            }

            return null;
        }

        private static async Task<(int ExitCode, string Output)> RunAsync(string executable, string filePath)
        {
            var startInfo = new ProcessStartInfo
            {
                FileName = executable,
                Arguments = $"-q \"{filePath}\"",
                UseShellExecute = false,
                CreateNoWindow = true,
                RedirectStandardOutput = true,
                RedirectStandardError = true,
            };

            string directory = Path.GetDirectoryName(filePath);
            if (!string.IsNullOrEmpty(directory))
            {
                startInfo.WorkingDirectory = directory;
            }

            using (var process = new System.Diagnostics.Process { StartInfo = startInfo })
            {
                process.Start();

                Task<string> stdout = process.StandardOutput.ReadToEndAsync();
                Task<string> stderr = process.StandardError.ReadToEndAsync();
                await Task.Run(() => process.WaitForExit());

                string output = string.Join(Environment.NewLine, new[] { await stdout, await stderr });
                Log.Info($"Ran stformat on {Path.GetFileName(filePath)} (exit {process.ExitCode})");
                if (!string.IsNullOrWhiteSpace(output))
                {
                    Log.Info($"stformat output: {output.Trim()}");
                }
                return (process.ExitCode, output.Trim());
            }
        }
    }
}
