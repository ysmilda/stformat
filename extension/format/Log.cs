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

        private static readonly object Sync = new object();

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
    }
}