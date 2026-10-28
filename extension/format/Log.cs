using System;
using System.IO;

namespace format
{
    internal static class Log
    {
        private static readonly string LogFile = Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
            "StFormat",
            "stformat.log");

        // The log is rotated to stformat.old.log once it passes this size.
        private const long MaxSize = 1024 * 1024;

        private static readonly object Sync = new object();

        private static bool _rotated;

        internal static void Info(string message)
        {
            Write("INFO", message);
        }

        internal static void Error(string message)
        {
            Write("ERROR", message);
        }

        internal static void Warning(string message)
        {
            Write("WARN", message);
        }

        private static void Write(string level, string message)
        {
            try
            {
                lock (Sync)
                {
                    Directory.CreateDirectory(Path.GetDirectoryName(LogFile));
                    Rotate();
                    File.AppendAllText(
                        LogFile,
                        string.Format("{0:yyyy-MM-dd HH:mm:ss.fff} [{1}] {2}{3}", DateTime.Now, level, message, Environment.NewLine));
                }
            }
            catch
            {
                // Logging must never break formatting.
            }
        }

        // Checked once per session; a single session does not log a megabyte.
        private static void Rotate()
        {
            if (_rotated)
            {
                return;
            }

            _rotated = true;
            var info = new FileInfo(LogFile);
            if (info.Exists && info.Length > MaxSize)
            {
                string old = Path.ChangeExtension(LogFile, ".old.log");
                File.Delete(old);
                File.Move(LogFile, old);
            }
        }
    }
}