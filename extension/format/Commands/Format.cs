using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Reflection;
using System.Runtime.InteropServices;
using System.Text;
using System.Text.RegularExpressions;
using System.Threading.Tasks;
using EnvDTE;
using Microsoft.VisualStudio;
using Microsoft.VisualStudio.ComponentModelHost;
using Microsoft.VisualStudio.Editor;
using Microsoft.VisualStudio.Shell.Interop;
using Microsoft.VisualStudio.Text;
using Microsoft.VisualStudio.Text.Editor;
using Microsoft.VisualStudio.Text.Operations;
using Microsoft.VisualStudio.TextManager.Interop;
using TCatSysManagerLib;

namespace format
{
    [Command("{030fb4a7-a21d-4b36-a28d-ffc41b84fbfc}", 0x0100)]
    internal sealed class Format : BaseCommand<Format>
    {
        // TwinCAT names the document of part of a PLC object after its file,
        // followed by "@method" (Visual Studio 2022, XAE Shell 4026) or by
        // ";Object\Folder\method" (XAE Shell 4024). This matches the file part.
        private static readonly Regex ObjectMoniker = new Regex(
            @"^(.*?\.(?:st|iecst|tcst|tcpou|tcgvl|tcdut|tcio))(?=[@;]|$)",
            RegexOptions.IgnoreCase | RegexOptions.CultureInvariant);

        // stformat formats a file in milliseconds. A run that takes this long
        // is stuck and is killed so the document is left alone.
        private const int TimeoutMilliseconds = 30000;

        protected override async Task ExecuteAsync(OleMenuCmdEventArgs e)
        {
            await ThreadHelper.JoinableTaskFactory.SwitchToMainThreadAsync();

            string executable = FindExecutable();
            if (executable is null)
            {
                await ShowMissingExecutableAsync();
                return;
            }

            Log.Info($"Using stformat: {executable}");

            // TwinCAT PLC objects (POUs and their methods/actions, GVLs, DUTs,
            // interfaces) may expose an automation model. Updating the object
            // keeps the change inside the environment, so the TwinCAT editor
            // does not report the file as modified externally.
            DTE dte = GetDTE();
            if (TryGetActivePlcObject(dte, out Document document, out ITcPlcDeclaration declaration, out ITcPlcImplementation implementation))
            {
                Log.Info("Formatting TwinCAT PLC object in place");
                await GuardAsync(document?.FullName, () => FormatPlcObjectAsync(executable, document, declaration, implementation, interactive: true));
                return;
            }

            DocumentView view = await VS.Documents.GetActiveDocumentViewAsync();

            await ThreadHelper.JoinableTaskFactory.SwitchToMainThreadAsync();
            string framePath = GetActiveDocumentPath();

            string moniker = PickSupportedDocument(view?.FilePath, framePath);
            if (moniker is null)
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

            Log.Info($"Formatting {moniker}");
            await GuardAsync(moniker, () => FormatDocumentAsync(executable, moniker, interactive: true));
        }

        internal static async Task ShowMissingExecutableAsync()
        {
            Log.Error("stformat.exe not found");
            await VS.MessageBox.ShowErrorAsync(
                "Format",
                "Could not find stformat.exe. Install it (for example with " +
                "'winget install --id Ysmilda.stformat -e') or place stformat.exe next to this extension.");
        }

        // Formats an open document: through the TwinCAT automation object
        // when it has one, otherwise in its text buffer.
        internal static async Task FormatDocumentAsync(string executable, string moniker, bool interactive)
        {
            await ThreadHelper.JoinableTaskFactory.SwitchToMainThreadAsync();

            DTE dte = GetDTE();
            if (TryGetPlcObject(dte, moniker, out Document document, out ITcPlcDeclaration declaration, out ITcPlcImplementation implementation))
            {
                Log.Info($"TwinCAT PLC object: {moniker}");
                await FormatPlcObjectAsync(executable, document, declaration, implementation, interactive);
                return;
            }

            OpenBuffer open = FindOpenBuffer(moniker);
            if (open is null)
            {
                Log.Warning($"No text buffer or TwinCAT object for {moniker}");
                if (interactive)
                {
                    await VS.MessageBox.ShowWarningAsync(
                        "Format",
                        $"{DisplayName(moniker)} is not open in a text editor or a TwinCAT editor, so it cannot be formatted here.");
                }
                return;
            }

            await FormatBufferAsync(executable, open, interactive);
        }

        // Formats a TwinCAT PLC object through the automation interface so the
        // change is applied to the open document instead of the file on disk.
        // The document is only saved again when formatting changed it.
        private static async Task FormatPlcObjectAsync(
            string executable,
            Document document,
            ITcPlcDeclaration declaration,
            ITcPlcImplementation implementation,
            bool interactive)
        {
            await ThreadHelper.JoinableTaskFactory.SwitchToMainThreadAsync();

            // Persist pending edits first so the automation model is current.
            // A document that was just saved is current already.
            if (interactive)
            {
                TrySave(document);
            }

            string name = document?.Name ?? "PLC object";
            bool changed = false;

            if (declaration is not null)
            {
                string original = declaration.DeclarationText;
                string formatted = await FormatTextAsync(executable, original, name, interactive);
                if (formatted is null)
                {
                    return;
                }

                await ThreadHelper.JoinableTaskFactory.SwitchToMainThreadAsync();
                changed |= ApplyPlcPart(name, "declaration", original, formatted, text =>
                {
                    ThreadHelper.ThrowIfNotOnUIThread();
                    declaration.DeclarationText = text;
                });
            }

            // Only Structured Text implementations can be formatted; other
            // languages (FBD, LD, ...) are left untouched.
            if (implementation is not null && IsStructuredText(implementation))
            {
                string original = implementation.ImplementationText;
                string formatted = await FormatTextAsync(executable, original, name, interactive);
                if (formatted is null)
                {
                    return;
                }

                await ThreadHelper.JoinableTaskFactory.SwitchToMainThreadAsync();
                changed |= ApplyPlcPart(name, "implementation", original, formatted, text =>
                {
                    ThreadHelper.ThrowIfNotOnUIThread();
                    implementation.ImplementationText = text;
                });
            }

            if (changed)
            {
                TrySave(document);
            }
            else
            {
                Log.Info($"No formatting changes for {name}");
            }
        }

        // Applies the formatted text of one part (declaration or
        // implementation) of a PLC object and reports whether it changed.
        // When the part is open in an editor, its buffer is edited, so Undo
        // reverts the formatting in one step; setting the automation property
        // replaces the text without an undo step. Must run on the UI thread.
        internal static bool ApplyPlcPart(string name, string part, string original, string formatted, Action<string> setText)
        {
            ThreadHelper.ThrowIfNotOnUIThread();

            string wrapped = WrapText(formatted, original);
            if (!Differs(wrapped, original))
            {
                return false;
            }

            if (TryFindPartBuffer(original, out ITextBuffer buffer, out ITextView view))
            {
                string current = buffer.CurrentSnapshot.GetText();
                ReplaceText(buffer, view, current, Reshape(formatted, current));
                Log.Info($"{name}: {part} reformatted in its editor");
                return true;
            }

            setText(wrapped);
            Log.Info($"{name}: {part} reformatted through the automation interface");
            return true;
        }

        // Finds the one open editor buffer that holds text. TwinCAT editors
        // show the declaration and the implementation in separate text views;
        // which object a view belongs to is not exposed, so a buffer is only
        // used when its text identifies it.
        private static bool TryFindPartBuffer(string text, out ITextBuffer buffer, out ITextView view)
        {
            ThreadHelper.ThrowIfNotOnUIThread();

            buffer = null;
            view = null;
            string wanted = Normalize(text);
            if (string.IsNullOrWhiteSpace(wanted))
            {
                // Every empty part looks alike.
                return false;
            }

            foreach (IWpfTextView candidate in TextViewTracker.OpenViews)
            {
                ITextBuffer documentBuffer = candidate.TextDataModel.DocumentBuffer;
                if (documentBuffer == buffer || Normalize(documentBuffer.CurrentSnapshot.GetText()) != wanted)
                {
                    continue;
                }

                if (buffer is not null)
                {
                    // Two buffers hold the same text; don't guess.
                    buffer = null;
                    view = null;
                    return false;
                }

                buffer = documentBuffer;
                view = candidate;
            }

            return buffer is not null;
        }

        // Gives formatted text the leading and trailing line breaks and the
        // line ending of shape, the text it replaces.
        private static string Reshape(string formatted, string shape)
        {
            string ending = shape.Contains("\r\n") ? "\r\n" : shape.Contains("\n") ? "\n" : "\r\n";

            int lead = 0;
            while (lead < shape.Length && (shape[lead] == '\r' || shape[lead] == '\n'))
            {
                lead++;
            }

            int trail = shape.Length;
            while (trail > lead && (shape[trail - 1] == '\r' || shape[trail - 1] == '\n'))
            {
                trail--;
            }

            string body = formatted.Trim('\r', '\n').Replace("\r\n", "\n");
            if (ending != "\n")
            {
                body = body.Replace("\n", ending);
            }

            return shape.Substring(0, lead) + body + shape.Substring(trail);
        }

        // An open document's text buffer, found through the running document
        // table. The XAE Shell 4024 keeps a POU method in a text buffer whose
        // moniker is not a file, so the editor's document lookup misses it.
        private sealed class OpenBuffer
        {
            public string Moniker;
            public ITextBuffer Buffer;
            public ITextView View;
            public ITextDocument Document;
            public IVsHierarchy Hierarchy;
            public uint ItemId;
            public uint Cookie;
        }

        private static OpenBuffer FindOpenBuffer(string moniker)
        {
            ThreadHelper.ThrowIfNotOnUIThread();

            var rdt = (IVsRunningDocumentTable)Microsoft.VisualStudio.Shell.Package.GetGlobalService(typeof(SVsRunningDocumentTable));
            var componentModel = (IComponentModel)Microsoft.VisualStudio.Shell.Package.GetGlobalService(typeof(SComponentModel));
            IVsEditorAdaptersFactoryService adapters = componentModel?.GetService<IVsEditorAdaptersFactoryService>();
            if (rdt is null || adapters is null)
            {
                return null;
            }

            IntPtr docData = IntPtr.Zero;
            try
            {
                if (rdt.FindAndLockDocument((uint)_VSRDTFLAGS.RDT_NoLock, moniker, out IVsHierarchy hierarchy, out uint itemId, out docData, out uint cookie) != VSConstants.S_OK ||
                    docData == IntPtr.Zero)
                {
                    return null;
                }

                object data = Marshal.GetObjectForIUnknown(docData);
                IVsTextBuffer vsBuffer = data as IVsTextBuffer;
                if (vsBuffer is null && data is IVsTextBufferProvider provider && provider.GetTextBuffer(out IVsTextLines lines) == VSConstants.S_OK)
                {
                    vsBuffer = lines;
                }

                ITextBuffer buffer = vsBuffer is null ? null : adapters.GetDocumentBuffer(vsBuffer);
                if (buffer is null)
                {
                    Log.Info($"Document data of {moniker} is not a text buffer");
                    return null;
                }

                buffer.Properties.TryGetProperty(typeof(ITextDocument), out ITextDocument document);
                return new OpenBuffer
                {
                    Moniker = moniker,
                    Buffer = buffer,
                    View = FindActiveView(adapters, buffer),
                    Document = document,
                    Hierarchy = hierarchy,
                    ItemId = itemId,
                    Cookie = cookie,
                };
            }
            finally
            {
                if (docData != IntPtr.Zero)
                {
                    Marshal.Release(docData);
                }
            }
        }

        // Returns the active text view when it shows buffer, so the caret can
        // be kept in place; null otherwise.
        private static ITextView FindActiveView(IVsEditorAdaptersFactoryService adapters, ITextBuffer buffer)
        {
            ThreadHelper.ThrowIfNotOnUIThread();

            var textManager = (IVsTextManager)Microsoft.VisualStudio.Shell.Package.GetGlobalService(typeof(SVsTextManager));
            if (textManager is null || textManager.GetActiveView(1, null, out IVsTextView active) != VSConstants.S_OK || active is null)
            {
                return null;
            }

            IWpfTextView view = adapters.GetWpfTextView(active);
            return view is not null && (view.TextBuffer == buffer || view.TextDataModel.DocumentBuffer == buffer) ? view : null;
        }

        private static void Save(OpenBuffer open)
        {
            ThreadHelper.ThrowIfNotOnUIThread();

            var rdt = (IVsRunningDocumentTable)Microsoft.VisualStudio.Shell.Package.GetGlobalService(typeof(SVsRunningDocumentTable));
            int hr = rdt?.SaveDocuments((uint)__VSRDTSAVEOPTIONS.RDTSAVEOPT_SaveIfDirty, open.Hierarchy, open.ItemId, open.Cookie) ?? VSConstants.E_FAIL;
            if (hr != VSConstants.S_OK)
            {
                Log.Warning($"Saving {open.Moniker} failed (hr 0x{hr:X8})");
            }
        }

        // Formats a document in its text buffer, so the editor never sees the
        // file change underneath it, the caret stays on its line and the
        // change is a single undo step. ST is formatted through stdin; TwinCAT
        // XML open as text is formatted from its file.
        private static async Task FormatBufferAsync(string executable, OpenBuffer open, bool interactive)
        {
            await ThreadHelper.JoinableTaskFactory.SwitchToMainThreadAsync();

            ITextBuffer buffer = open.Buffer;
            string name = DisplayName(open.Moniker);
            string original = buffer.CurrentSnapshot.GetText();

            string formatted;
            if (original.TrimStart().StartsWith("<", StringComparison.Ordinal))
            {
                string filePath = open.Document?.FilePath;
                if (!IsSupported(filePath) || !File.Exists(filePath))
                {
                    Log.Info($"{name} holds XML but is not a TwinCAT file; skipped");
                    return;
                }

                // The XML is read from disk, so the file must match the buffer.
                if (open.Document.IsDirty)
                {
                    Save(open);
                }

                byte[] output = await RunAsync(executable, $"-q -stdout \"{filePath}\"", null, name, interactive);
                if (output is null)
                {
                    return;
                }

                formatted = Decode(output, open.Document.Encoding);
            }
            else
            {
                formatted = await FormatTextAsync(executable, original, name, interactive);
                if (formatted is null)
                {
                    return;
                }
            }

            await ThreadHelper.JoinableTaskFactory.SwitchToMainThreadAsync();

            // The buffer may have been edited while stformat ran; the result
            // belongs to the old text and is dropped.
            if (buffer.CurrentSnapshot.GetText() != original)
            {
                Log.Warning($"{name} changed while formatting; result dropped");
                return;
            }

            if (formatted == original)
            {
                Log.Info($"No formatting changes for {name}");
                return;
            }

            ReplaceText(buffer, open.View, original, formatted);
            Save(open);
            Log.Info($"{name} formatted in its buffer");
        }

        // Replaces only the part of the buffer that changed and keeps the caret
        // on the line it was on.
        private static void ReplaceText(ITextBuffer buffer, ITextView view, string original, string formatted)
        {
            ThreadHelper.ThrowIfNotOnUIThread();

            int start = 0;
            int limit = Math.Min(original.Length, formatted.Length);
            while (start < limit && original[start] == formatted[start])
            {
                start++;
            }

            int originalEnd = original.Length;
            int formattedEnd = formatted.Length;
            while (originalEnd > start && formattedEnd > start && original[originalEnd - 1] == formatted[formattedEnd - 1])
            {
                originalEnd--;
                formattedEnd--;
            }

            int caretLine = -1;
            int caretColumn = 0;
            if (view is not null)
            {
                SnapshotPoint caret = view.Caret.Position.BufferPosition;
                ITextSnapshotLine line = caret.GetContainingLine();
                caretLine = line.LineNumber;
                caretColumn = caret.Position - line.Start.Position;
            }

            // One named transaction, so a single Undo reverts the formatting.
            ITextUndoTransaction transaction = GetUndoHistory(buffer)?.CreateTransaction("Format Structured Text");
            try
            {
                using (ITextEdit edit = buffer.CreateEdit())
                {
                    edit.Replace(start, originalEnd - start, formatted.Substring(start, formattedEnd - start));
                    edit.Apply();
                }

                transaction?.Complete();
            }
            finally
            {
                transaction?.Dispose();
            }

            if (caretLine >= 0)
            {
                // The caret lives in the view's buffer, which can be a
                // projection of the document buffer that was edited.
                ITextSnapshot snapshot = view.TextBuffer.CurrentSnapshot;
                ITextSnapshotLine line = snapshot.GetLineFromLineNumber(Math.Min(caretLine, snapshot.LineCount - 1));
                view.Caret.MoveTo(new SnapshotPoint(snapshot, line.Start.Position + Math.Min(caretColumn, line.Length)));
            }
        }

        private static ITextUndoHistory GetUndoHistory(ITextBuffer buffer)
        {
            ThreadHelper.ThrowIfNotOnUIThread();

            var componentModel = (IComponentModel)Microsoft.VisualStudio.Shell.Package.GetGlobalService(typeof(SComponentModel));
            ITextUndoHistoryRegistry registry = componentModel?.GetService<ITextUndoHistoryRegistry>();
            return registry is not null && registry.TryGetHistory(buffer, out ITextUndoHistory history) ? history : null;
        }

        // Monikers of the documents being formatted. Formatting saves the
        // document, which raises the save event again; that event is ignored.
        // Other documents saved at the same time (Save All) are still
        // formatted. Only touched on the UI thread.
        private static readonly HashSet<string> Formatting = new HashSet<string>(StringComparer.OrdinalIgnoreCase);

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
            // Projects, solutions and other files are saved all the time;
            // ignore them before touching the automation model.
            if (!IsSupported(moniker) || Formatting.Contains(moniker))
            {
                return;
            }

            if (!General.Instance.FormatOnSave)
            {
                return;
            }

            Log.Info($"Document saved: {moniker}");
            _toolkitThreadHelper.JoinableTaskFactory.RunAsync(async () =>
            {
                try
                {
                    await _toolkitThreadHelper.JoinableTaskFactory.SwitchToMainThreadAsync(_toolkitThreadHelper.DisposalToken);
                    await GuardAsync(moniker, () => FormatSavedDocumentAsync(moniker));
                }
                catch (Exception ex)
                {
                    Log.Error($"Format-on-save failed for {moniker}: {ex}");
                }
            }).FireAndForget();
        }

        private static async Task FormatSavedDocumentAsync(string moniker)
        {
            await ThreadHelper.JoinableTaskFactory.SwitchToMainThreadAsync();

            string executable = FindExecutable();
            if (executable is null)
            {
                Log.Warning("Format-on-save skipped: stformat.exe not found");
                return;
            }

            await FormatDocumentAsync(executable, moniker, interactive: false);
        }

        // Finds the automation object of an open document by its moniker.
        // TwinCAT method documents carry a suffix in the moniker, which is why
        // the document is matched instead of the file.
        private static bool TryGetPlcObject(
            DTE dte,
            string moniker,
            out Document document,
            out ITcPlcDeclaration declaration,
            out ITcPlcImplementation implementation)
        {
            ThreadHelper.ThrowIfNotOnUIThread();

            document = null;
            declaration = null;
            implementation = null;

            if (dte is null)
            {
                return false;
            }

            try
            {
                foreach (Document candidate in dte.Documents)
                {
                    if (!IsSamePath(candidate.FullName, moniker))
                    {
                        continue;
                    }

                    document = candidate;
                    return TryGetPlcInterfaces(
                        () =>
                        {
                            ThreadHelper.ThrowIfNotOnUIThread();
                            return candidate.ProjectItem?.Object;
                        },
                        out declaration,
                        out implementation);
                }
            }
            catch (Exception)
            {
                // Fall through; not a PLC object.
            }

            return false;
        }

        // Tries to obtain the TwinCAT automation object for the active
        // window. Returns true when it is a PLC object (POU, GVL, DUT or
        // interface) that can be formatted in place.
        private static bool TryGetActivePlcObject(
            DTE dte,
            out Document document,
            out ITcPlcDeclaration declaration,
            out ITcPlcImplementation implementation)
        {
            ThreadHelper.ThrowIfNotOnUIThread();

            document = null;
            declaration = null;
            implementation = null;

            if (dte is null)
            {
                return false;
            }

            if (!TryGetPlcInterfaces(
                () =>
                {
                    ThreadHelper.ThrowIfNotOnUIThread();
                    return dte.ActiveWindow?.ProjectItem?.Object;
                },
                out declaration,
                out implementation))
            {
                return false;
            }

            try
            {
                document = dte.ActiveWindow?.Document ?? dte.ActiveDocument;
            }
            catch (Exception)
            {
                // Formatting still works; only saving afterwards is skipped.
            }

            return true;
        }

        private static bool TryGetPlcInterfaces(
            Func<object> getAutomation,
            out ITcPlcDeclaration declaration,
            out ITcPlcImplementation implementation)
        {
            declaration = null;
            implementation = null;

            object automation;
            try
            {
                automation = getAutomation();
            }
            catch (Exception)
            {
                return false;
            }

            declaration = automation as ITcPlcDeclaration;
            implementation = automation as ITcPlcImplementation;
            return declaration is not null || implementation is not null;
        }

        // Runs action unless the document is already being formatted. Callers
        // are on the UI thread, so the switch completes synchronously and the
        // document is marked before the save event of its own save arrives.
        private static async Task GuardAsync(string moniker, Func<Task> action)
        {
            await ThreadHelper.JoinableTaskFactory.SwitchToMainThreadAsync();

            string key = moniker ?? string.Empty;
            if (!Formatting.Add(key))
            {
                return;
            }

            try
            {
                await action();
            }
            finally
            {
                await ThreadHelper.JoinableTaskFactory.SwitchToMainThreadAsync();
                Formatting.Remove(key);
            }
        }

        // Formats a piece of ST text through stformat's standard input.
        // Returns null and reports the error when stformat fails.
        private static async Task<string> FormatTextAsync(string executable, string text, string name, bool interactive)
        {
            (string formatted, string error) = await TryFormatTextAsync(executable, text, name);
            if (error is not null)
            {
                await ReportAsync(name, error, interactive);
            }

            return formatted;
        }

        // Formats a piece of ST text through stformat's standard input and
        // returns either the result or the error. Safe to call from any thread.
        internal static async Task<(string Text, string Error)> TryFormatTextAsync(string executable, string text, string name)
        {
            if (string.IsNullOrEmpty(text))
            {
                return (text, null);
            }

            var utf8 = new UTF8Encoding(false);
            (byte[] output, string error) = await TryRunAsync(executable, "-q -stdin", utf8.GetBytes(text), name);
            return output is null ? (null, error) : (utf8.GetString(output), null);
        }

        // Runs stformat and returns its standard output. Returns null and
        // reports the error when it cannot be run or fails.
        private static async Task<byte[]> RunAsync(string executable, string arguments, byte[] input, string name, bool interactive)
        {
            (byte[] output, string error) = await TryRunAsync(executable, arguments, input, name);
            if (error is not null)
            {
                await ReportAsync(name, error, interactive);
            }

            return output;
        }

        // Runs stformat and returns its standard output, or the error when it
        // cannot be run or fails. Safe to call from any thread.
        internal static async Task<(byte[] Output, string Error)> TryRunAsync(string executable, string arguments, byte[] input, string name)
        {
            string error;
            try
            {
                (int exitCode, byte[] output, string stderr) = await Task.Run(() => RunProcessAsync(executable, arguments, input));
                Log.Info($"Ran stformat {arguments} for {name} (exit {exitCode})");
                if (exitCode == 0)
                {
                    return (output, null);
                }

                error = string.IsNullOrEmpty(stderr) ? $"stformat exited with code {exitCode}." : stderr;
            }
            catch (Exception ex)
            {
                error = $"Failed to run stformat: {ex.Message}";
            }

            Log.Error($"{name}: {error}");
            return (null, error);
        }

        // Only an explicit command shows a message box; format-on-save uses
        // the status bar so a save is never interrupted by a dialog.
        private static async Task ReportAsync(string name, string error, bool interactive)
        {
            await ThreadHelper.JoinableTaskFactory.SwitchToMainThreadAsync();
            if (interactive)
            {
                await VS.MessageBox.ShowErrorAsync("Format", $"Could not format {name}:{Environment.NewLine}{error}");
            }
            else
            {
                string firstLine = error.Split('\n')[0].Trim();
                await VS.StatusBar.ShowMessageAsync($"StFormat: {name} not formatted: {firstLine}");
            }
        }

        private static async Task<(int ExitCode, byte[] Output, string Error)> RunProcessAsync(string executable, string arguments, byte[] input)
        {
            var startInfo = new ProcessStartInfo
            {
                FileName = executable,
                Arguments = arguments,
                UseShellExecute = false,
                CreateNoWindow = true,
                RedirectStandardInput = input is not null,
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                StandardErrorEncoding = Encoding.UTF8,
            };

            using (var process = new System.Diagnostics.Process { StartInfo = startInfo })
            {
                process.Start();

                // Both pipes are drained while the input is written, so a large
                // file cannot fill a pipe and deadlock the two processes.
                Task<byte[]> stdout = Task.Run(() =>
                {
                    using (var buffer = new MemoryStream())
                    {
                        process.StandardOutput.BaseStream.CopyTo(buffer);
                        return buffer.ToArray();
                    }
                });
                Task<string> stderr = process.StandardError.ReadToEndAsync();

                if (input is not null)
                {
                    try
                    {
                        using (Stream stdin = process.StandardInput.BaseStream)
                        {
                            await stdin.WriteAsync(input, 0, input.Length);
                        }
                    }
                    catch (IOException)
                    {
                        // stformat exited early; its exit code and stderr say why.
                    }
                }

                if (!process.WaitForExit(TimeoutMilliseconds))
                {
                    try
                    {
                        process.Kill();
                    }
                    catch (InvalidOperationException)
                    {
                        // Exited in the meantime.
                    }

                    throw new TimeoutException($"stformat did not finish within {TimeoutMilliseconds / 1000} seconds.");
                }

                return (process.ExitCode, await stdout, (await stderr).Trim());
            }
        }

        // Decodes a file's bytes with the encoding the editor uses for it,
        // without the byte order mark, which the buffer does not hold.
        private static string Decode(byte[] bytes, Encoding encoding)
        {
            byte[] preamble = encoding.GetPreamble();
            int offset = 0;
            if (preamble.Length > 0 && bytes.Length >= preamble.Length)
            {
                offset = preamble.Length;
                for (int i = 0; i < preamble.Length; i++)
                {
                    if (bytes[i] != preamble[i])
                    {
                        offset = 0;
                        break;
                    }
                }
            }

            return encoding.GetString(bytes, offset, bytes.Length - offset);
        }

        internal static DTE GetDTE()
        {
            ThreadHelper.ThrowIfNotOnUIThread();
            return (DTE)Microsoft.VisualStudio.Shell.Package.GetGlobalService(typeof(DTE));
        }

        internal static bool IsStructuredText(ITcPlcImplementation implementation)
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

        private static void TrySave(Document document)
        {
            ThreadHelper.ThrowIfNotOnUIThread();
            try
            {
                document?.Save("");
            }
            catch (Exception)
            {
                // The document may not be saveable; the change stays in memory.
            }
        }

        // Compares two pieces of ST text ignoring line ending style and
        // leading and trailing line breaks, so text the editor normalises on
        // assignment is not set, and saved, again on every save.
        private static bool Differs(string left, string right)
        {
            return Normalize(left) != Normalize(right);
        }

        // The file based formatter stores the declaration and implementation
        // content with a single leading and trailing line break inside the
        // CDATA section, in the line ending of the block. Reproduce that so
        // the automation path produces the same output and never mixes line
        // endings.
        private static string WrapText(string formatted, string original)
        {
            string ending = original is not null && original.Contains("\n") && !original.Contains("\r\n") ? "\n" : "\r\n";
            string body = formatted.Trim('\r', '\n').Replace("\r\n", "\n");
            if (ending != "\n")
            {
                body = body.Replace("\n", ending);
            }

            return ending + body + ending;
        }

        private static string Normalize(string text)
        {
            return text?.Replace("\r\n", "\n").Trim('\n');
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

        // Reports whether a file, or the PLC object a TwinCAT moniker points
        // into, has a supported extension.
        internal static bool IsSupported(string moniker)
        {
            return !string.IsNullOrEmpty(moniker) && ObjectMoniker.IsMatch(moniker);
        }

        // A short name for messages: "MAIN.TcPOU" or "MAIN.TcPOU (doExecute)".
        internal static string DisplayName(string moniker)
        {
            Match match = ObjectMoniker.Match(moniker ?? string.Empty);
            if (!match.Success)
            {
                return moniker;
            }

            string file = match.Groups[1].Value;
            int separator = file.LastIndexOfAny(new[] { '\\', '/' });
            string name = file.Substring(separator + 1);
            if (moniker.Length == file.Length)
            {
                return name;
            }

            string part = moniker.Substring(file.Length + 1);
            return $"{name} ({part.Substring(part.LastIndexOfAny(new[] { '\\', '/' }) + 1)})";
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

        internal static string FindExecutable()
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

                string candidate;
                try
                {
                    candidate = Path.Combine(directory.Trim().Trim('"'), "stformat.exe");
                }
                catch (ArgumentException)
                {
                    // Invalid characters in a PATH entry.
                    continue;
                }

                if (File.Exists(candidate))
                {
                    return candidate;
                }
            }

            return null;
        }
    }
}
