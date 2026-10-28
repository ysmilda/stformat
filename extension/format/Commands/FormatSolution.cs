using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text;
using System.Text.RegularExpressions;
using System.Threading.Tasks;
using System.Xml.Linq;
using EnvDTE;
using Project = EnvDTE.Project;
using Solution = EnvDTE.Solution;

namespace format
{
    // Formats every supported file of the solution on disk: the PLC objects
    // of the TwinCAT projects and the supported files of other projects.
    [Command("{030fb4a7-a21d-4b36-a28d-ffc41b84fbfc}", 0x0101)]
    internal sealed class FormatSolution : BaseCommand<FormatSolution>
    {
        private const string Title = "Format Solution";

        // EnvDTE80.ProjectKinds.vsProjectKindSolutionFolder
        private const string SolutionFolderKind = "{66A26720-8FB5-11D2-AA7E-00C04F688DDE}";

        private static readonly string[] TwinCatProjectExtensions = { ".tsproj", ".tspproj" };

        // A PLC project referenced from a TwinCAT project or its .xti files.
        private static readonly Regex PlcProjectReference = new Regex(
            "PrjFilePath\\s*=\\s*\"([^\"]+)\"",
            RegexOptions.IgnoreCase | RegexOptions.CultureInvariant);

        // At most this many failures are listed in the summary; the log has all.
        private const int ListedErrors = 15;

        protected override async Task ExecuteAsync(OleMenuCmdEventArgs e)
        {
            await ThreadHelper.JoinableTaskFactory.SwitchToMainThreadAsync();

            string executable = Format.FindExecutable();
            if (executable is null)
            {
                await Format.ShowMissingExecutableAsync();
                return;
            }

            DTE dte = Format.GetDTE();
            if (dte?.Solution is null || !dte.Solution.IsOpen)
            {
                await VS.MessageBox.ShowWarningAsync(Title, "There is no open solution to format.");
                return;
            }

            Format.SuppressOnSave = true;
            try
            {
                await FormatAsync(executable, dte);
            }
            finally
            {
                await ThreadHelper.JoinableTaskFactory.SwitchToMainThreadAsync();
                Format.SuppressOnSave = false;
                await VS.StatusBar.ClearAsync();
            }
        }

        private static async Task FormatAsync(string executable, DTE dte)
        {
            await ThreadHelper.JoinableTaskFactory.SwitchToMainThreadAsync();

            var files = new List<string>();
            bool twinCat = false;
            foreach (Project project in Projects(dte.Solution))
            {
                string projectFile = ProjectFile(project);
                if (projectFile is not null && Array.IndexOf(TwinCatProjectExtensions, Path.GetExtension(projectFile).ToLowerInvariant()) >= 0)
                {
                    twinCat = true;
                    foreach (string plcProject in PlcProjects(projectFile))
                    {
                        AddPlcProjectFiles(plcProject, files);
                    }
                }
                else if (projectFile is not null && string.Equals(Path.GetExtension(projectFile), ".plcproj", StringComparison.OrdinalIgnoreCase))
                {
                    twinCat = true;
                    AddPlcProjectFiles(projectFile, files);
                }
                else
                {
                    AddProjectItemFiles(project.ProjectItems, files);
                }
            }

            Log.Info($"Format Solution: {files.Count} files, TwinCAT: {twinCat}");
            if (files.Count == 0)
            {
                await VS.MessageBox.ShowWarningAsync(Title, "The solution has no files that stformat formats.");
                return;
            }

            // TwinCAT holds the PLC project in memory and would overwrite, or
            // ask about, every file changed underneath it. The solution is
            // closed while the files are formatted and opened again after.
            string solutionPath = dte.Solution.FullName;
            if (twinCat)
            {
                bool proceed = await VS.MessageBox.ShowConfirmAsync(
                    Title,
                    $"Format {files.Count} files on disk?{Environment.NewLine}{Environment.NewLine}" +
                    "All changes are saved and the solution is closed while the files are formatted, then opened again. " +
                    "Formatting cannot be undone with Undo; use source control to revert it.");
                if (!proceed)
                {
                    return;
                }

                dte.ExecuteCommand("File.SaveAll");
                dte.Solution.Close(true);
            }
            else
            {
                dte.ExecuteCommand("File.SaveAll");
            }

            var errors = new List<string>();
            int changed;
            try
            {
                changed = await FormatFilesAsync(executable, files, errors);
            }
            finally
            {
                if (twinCat)
                {
                    await ThreadHelper.JoinableTaskFactory.SwitchToMainThreadAsync();
                    await VS.StatusBar.ShowMessageAsync("StFormat: opening the solution again...");
                    dte.Solution.Open(solutionPath);
                }
            }

            await ThreadHelper.JoinableTaskFactory.SwitchToMainThreadAsync();
            var summary = new StringBuilder();
            summary.AppendLine($"{changed} of {files.Count} files reformatted.");
            if (errors.Count > 0)
            {
                summary.AppendLine();
                summary.AppendLine($"Not formatted ({errors.Count}):");
                foreach (string error in errors.Take(ListedErrors))
                {
                    summary.AppendLine(error);
                }
                if (errors.Count > ListedErrors)
                {
                    summary.AppendLine($"... and {errors.Count - ListedErrors} more, see %LOCALAPPDATA%\\StFormat\\stformat.log.");
                }
            }

            Log.Info($"Format Solution: {changed} of {files.Count} files reformatted, {errors.Count} failed");
            if (errors.Count > 0)
            {
                await VS.MessageBox.ShowWarningAsync(Title, summary.ToString());
            }
            else
            {
                await VS.MessageBox.ShowAsync(Title, summary.ToString());
            }
        }

        // Formats the files with as few stformat runs as the command line
        // allows; stformat formats the files of a run in parallel. Returns how
        // many files changed.
        private static async Task<int> FormatFilesAsync(string executable, List<string> files, List<string> errors)
        {
            int done = 0;
            int changed = 0;
            foreach (List<string> batch in Batches(files))
            {
                changed += await FormatBatchAsync(executable, batch, errors);
                done += batch.Count;
                await VS.StatusBar.ShowProgressAsync($"StFormat: formatted {done} of {files.Count} files", done, files.Count);
            }

            errors.Sort(StringComparer.OrdinalIgnoreCase);
            return changed;
        }

        // stformat writes the files of a run only when all of them format, so
        // a run that fails is repeated once without the files its error names.
        private static async Task<int> FormatBatchAsync(string executable, List<string> batch, List<string> errors)
        {
            (string output, string error) = await RunAsync(executable, batch);
            if (error is null)
            {
                return CountFormatted(output);
            }

            List<string> failed = batch.Where(file => error.IndexOf(file + ":", StringComparison.OrdinalIgnoreCase) >= 0).ToList();
            if (failed.Count == 0)
            {
                // The error names no file of the run, so none was written.
                errors.Add($"{batch.Count} files not formatted: {FirstLine(error)}");
                return 0;
            }

            foreach (string file in failed)
            {
                errors.Add($"{Path.GetFileName(file)}: {ErrorFor(file, error)}");
            }

            List<string> rest = batch.Except(failed, StringComparer.OrdinalIgnoreCase).ToList();
            if (rest.Count == 0)
            {
                return 0;
            }

            (output, error) = await RunAsync(executable, rest);
            if (error is null)
            {
                return CountFormatted(output);
            }

            errors.Add($"{rest.Count} files not formatted: {FirstLine(error)}");
            return 0;
        }

        private static async Task<(string Output, string Error)> RunAsync(string executable, List<string> files)
        {
            string arguments = string.Join(" ", files.Select(file => $"\"{file}\""));
            (byte[] output, string error) = await Format.TryRunAsync(executable, arguments, null, $"{files.Count} files");
            return (output is null ? null : Encoding.UTF8.GetString(output), error);
        }

        // Without -q stformat lists every file it changed as "formatted: <file>".
        private static int CountFormatted(string output)
        {
            return output.Split('\n').Count(line => line.StartsWith("formatted: ", StringComparison.Ordinal));
        }

        // The line of stformat's error output about one file, without the
        // file name and the "stformat: " prefix.
        private static string ErrorFor(string file, string error)
        {
            string line = error.Split('\n').FirstOrDefault(l => l.IndexOf(file + ":", StringComparison.OrdinalIgnoreCase) >= 0) ?? error;
            int at = line.IndexOf(file + ":", StringComparison.OrdinalIgnoreCase);
            return (at < 0 ? line : line.Substring(at + file.Length + 1)).Trim();
        }

        private static string FirstLine(string text)
        {
            return text.Split('\n')[0].Trim();
        }

        // Splits the files into runs whose command line stays well below the
        // 32767 characters Windows allows.
        private static IEnumerable<List<string>> Batches(List<string> files)
        {
            const int MaxArgumentsLength = 30000;
            var batch = new List<string>();
            int length = 0;
            foreach (string file in files)
            {
                int add = file.Length + 3;
                if (batch.Count > 0 && length + add > MaxArgumentsLength)
                {
                    yield return batch;
                    batch = new List<string>();
                    length = 0;
                }

                batch.Add(file);
                length += add;
            }

            if (batch.Count > 0)
            {
                yield return batch;
            }
        }

        // The projects of the solution, including those in solution folders.
        private static List<Project> Projects(Solution solution)
        {
            ThreadHelper.ThrowIfNotOnUIThread();

            var projects = new List<Project>();
            foreach (Project project in solution.Projects)
            {
                AddProject(project, projects);
            }

            return projects;
        }

        private static void AddProject(Project project, List<Project> projects)
        {
            ThreadHelper.ThrowIfNotOnUIThread();

            if (project is null)
            {
                return;
            }

            string kind;
            try
            {
                kind = project.Kind;
            }
            catch (Exception)
            {
                return;
            }

            if (!string.Equals(kind, SolutionFolderKind, StringComparison.OrdinalIgnoreCase))
            {
                projects.Add(project);
                return;
            }

            foreach (ProjectItem item in project.ProjectItems)
            {
                AddProject(item.SubProject, projects);
            }
        }

        private static string ProjectFile(Project project)
        {
            ThreadHelper.ThrowIfNotOnUIThread();

            try
            {
                string file = project.FullName;
                return string.IsNullOrEmpty(file) || !File.Exists(file) ? null : file;
            }
            catch (Exception)
            {
                return null;
            }
        }

        // The PLC projects (.plcproj) a TwinCAT project references, either in
        // the .tsproj itself or, with independent project files, in its .xti
        // files. Paths are relative to the TwinCAT project.
        private static IEnumerable<string> PlcProjects(string twinCatProject)
        {
            string directory = Path.GetDirectoryName(twinCatProject);
            var sources = new List<string> { twinCatProject };
            try
            {
                sources.AddRange(Directory.EnumerateFiles(directory, "*.xti", SearchOption.AllDirectories));
            }
            catch (Exception ex)
            {
                Log.Warning($"Format Solution: cannot list .xti files of {twinCatProject}: {ex.Message}");
            }

            var found = new List<string>();
            foreach (string source in sources)
            {
                string text;
                try
                {
                    text = File.ReadAllText(source);
                }
                catch (Exception ex)
                {
                    Log.Warning($"Format Solution: cannot read {source}: {ex.Message}");
                    continue;
                }

                foreach (Match match in PlcProjectReference.Matches(text))
                {
                    string relative = System.Net.WebUtility.HtmlDecode(match.Groups[1].Value);
                    string plcProject = new[] { directory, Path.GetDirectoryName(source) }
                        .Select(baseDirectory => Path.GetFullPath(Path.Combine(baseDirectory, relative)))
                        .FirstOrDefault(File.Exists);
                    if (plcProject is null)
                    {
                        Log.Warning($"Format Solution: PLC project {relative} of {source} not found");
                    }
                    else if (!found.Contains(plcProject, StringComparer.OrdinalIgnoreCase))
                    {
                        found.Add(plcProject);
                    }
                }
            }

            return found;
        }

        // Adds the supported files a PLC project compiles.
        private static void AddPlcProjectFiles(string plcProject, List<string> files)
        {
            string directory = Path.GetDirectoryName(plcProject);
            try
            {
                foreach (XElement item in XDocument.Load(plcProject).Descendants().Where(element => element.Name.LocalName == "Compile"))
                {
                    string include = (string)item.Attribute("Include");
                    if (!string.IsNullOrEmpty(include))
                    {
                        AddFile(Path.GetFullPath(Path.Combine(directory, Uri.UnescapeDataString(include))), files);
                    }
                }
            }
            catch (Exception ex)
            {
                Log.Warning($"Format Solution: cannot read {plcProject}: {ex.Message}");
            }
        }

        private static void AddProjectItemFiles(ProjectItems items, List<string> files)
        {
            ThreadHelper.ThrowIfNotOnUIThread();

            if (items is null)
            {
                return;
            }

            foreach (ProjectItem item in items)
            {
                try
                {
                    for (short i = 1; i <= item.FileCount; i++)
                    {
                        AddFile(item.FileNames[i], files);
                    }

                    AddProjectItemFiles(item.ProjectItems, files);
                }
                catch (Exception)
                {
                    // Not a file item; skip it.
                }
            }
        }

        private static void AddFile(string file, List<string> files)
        {
            if (Format.IsSupported(file) && File.Exists(file) && !files.Contains(file, StringComparer.OrdinalIgnoreCase))
            {
                files.Add(file);
            }
        }
    }
}
