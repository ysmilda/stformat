using System;
using System.Collections.Generic;
using System.Reflection;
using System.Runtime.CompilerServices;
using System.Runtime.InteropServices;
using Microsoft.VisualStudio;
using Microsoft.VisualStudio.Shell;
using Microsoft.VisualStudio.Shell.Interop;

namespace format
{
    // Writes text into a TwinCAT PLC object through the buffer that the
    // TwinCAT editor itself edits. That buffer records the write as an undo
    // action, so Ctrl-Z in the TwinCAT editor reverts the format, which the
    // automation interface (a plain property setter) does not do.
    //
    // The buffer lives in TwinCATPlcControl.dll and is not part of a
    // published TwinCAT API, so every step below is reflection and every
    // step is optional: TryWrite returns false as soon as one of them is
    // missing and the caller falls back to the automation interface.
    //
    // The write is checked before and after it happens. Both checks are
    // strict enough that a member whose meaning differs from what this code
    // assumes stops the write instead of corrupting the object. The only
    // check that touches the buffer is the probe, and its residue is always
    // overwritten, either by the write that follows it or by the automation
    // fallback of the caller.
    internal static class PlcUndoBuffer
    {
        private const BindingFlags Members =
            BindingFlags.Instance | BindingFlags.Public | BindingFlags.NonPublic;

        private const string WrapperTypeName = "Beckhoff.TwinCAT.VS.TextImage.CDSTextBufferWrapper";
        private const string ManagerTypeName = "Beckhoff.TwinCAT.VS.TextImage.TextBufferManager";
        private const string TwinCatNamespace = "Beckhoff.TwinCAT.VS.";

        // The search for the buffer stays inside TwinCAT objects and does not
        // walk a whole document graph.
        private const int MaxNodes = 200;
        private const int MaxDepth = 6;

        // Two blank lines at the top of the buffer. A write that addresses
        // whole lines puts them at the start; anything else puts them
        // somewhere else, which is how the signature is verified.
        private const string ProbeText = "\n\n";

        // Replaces the whole content of the PLC object buffer with text.
        // originalText must be the text the caller read from the object and
        // formattedText the result; both are used to verify the write.
        internal static bool TryWrite(string moniker, string originalText, string formattedText, out string reason)
        {
            ThreadHelper.ThrowIfNotOnUIThread();
            reason = null;

            object wrapper = TryFindBuffer(moniker, out reason);
            if (wrapper is null)
            {
                return false;
            }

            // The buffer of a POU holds the declaration and the
            // implementation together, so a whole buffer write would only be
            // correct when the caller formatted everything it holds.
            string buffered = ReadText(wrapper);
            if (buffered is null)
            {
                reason = "the editor buffer has no text";
                return false;
            }

            if (!SameText(buffered, originalText))
            {
                reason = "the editor buffer holds more than the formatted text";
                return false;
            }

            if (!Probe(wrapper, buffered, out reason))
            {
                return false;
            }

            // The line count is read after the probe, because the probe left
            // its two lines in the buffer and the write below has to cover
            // them as well.
            int lines = Call(wrapper, "LineCount") is int count ? count : -1;
            if (lines < 0)
            {
                reason = "the editor buffer has no line count";
                return false;
            }

            if (!Write(wrapper, lines, formattedText, out reason))
            {
                return false;
            }

            string after = ReadText(wrapper);
            if (!SameText(after, formattedText))
            {
                reason = "the editor buffer does not hold the formatted text after the write";
                return false;
            }

            return true;
        }

        // Fetches the TwinCAT text buffer of a running PLC document. The
        // document data of the document is the only supported entry point to
        // it, and what that data holds differs between TwinCAT versions, so
        // the buffer is searched for instead of assumed.
        private static object TryFindBuffer(string moniker, out string reason)
        {
            reason = null;

            object data = GetDocumentData(moniker);
            if (data is null)
            {
                reason = "the document is not in the running document table";
                return null;
            }

            var queue = new Queue<(object Node, int Depth)>();
            var seen = new HashSet<object>(ReferenceComparer.Instance);
            queue.Enqueue((data, 0));

            while (queue.Count > 0 && seen.Count < MaxNodes)
            {
                (object node, int depth) = queue.Dequeue();
                if (node is null || !seen.Add(node))
                {
                    continue;
                }

                string name = node.GetType().FullName;
                if (name == WrapperTypeName)
                {
                    return node;
                }

                if (depth >= MaxDepth || (name != ManagerTypeName && !IsTwinCatObject(node)))
                {
                    continue;
                }

                foreach (object child in Children(node))
                {
                    queue.Enqueue((child, depth + 1));
                }
            }

            reason = $"{data.GetType().FullName} holds no {WrapperTypeName}";
            return null;
        }

        // Writes text that has to come back unchanged, which is how the
        // meaning of the address arguments is verified. Two blank lines at
        // the start of the buffer are the visible result of addressing text
        // by line.
        private static bool Probe(object wrapper, string expected, out string reason)
        {
            reason = null;

            if (!Write(wrapper, 0, ProbeText, out reason))
            {
                return false;
            }

            string after = ReadText(wrapper);
            if (after is null || !SameText(after, ProbeText + expected))
            {
                reason = "the editor buffer does not address text by line";
                return false;
            }

            return true;
        }

        // Replaces lineCount lines starting at line one. Returns false when
        // any step is missing or refused the write.
        private static bool Write(object wrapper, int lineCount, string text, out string reason)
        {
            reason = null;

            object editable = Call(wrapper, "GetBufferToModify");
            if (editable is null)
            {
                reason = "the editor buffer has no editable copy";
                return false;
            }

            try
            {
                // The compound action is what puts the write on the undo
                // stack of the TwinCAT editor.
                if (!Invoke(editable, "BeginCompoundAction"))
                {
                    reason = "the editable buffer cannot open an undo action";
                    return false;
                }

                if (!Invoke(editable, "Replace", 1, lineCount, text))
                {
                    reason = "the editable buffer cannot replace text";
                    return false;
                }

                if (!Invoke(editable, "Apply", true))
                {
                    reason = "the editable buffer refused the write";
                    return false;
                }

                // Closing the action is best effort: the write itself is done.
                Invoke(editable, "EndCompoundAction");
            }
            catch (Exception ex)
            {
                reason = ex.Message;
                return false;
            }
            finally
            {
                Dispose(editable);
            }

            return true;
        }

        private static string ReadText(object wrapper)
        {
            return Call(wrapper, "GetText") as string;
        }

        private static object GetDocumentData(string moniker)
        {
            try
            {
                var rdt = (IVsRunningDocumentTable)Microsoft.VisualStudio.Shell.Package
                    .GetGlobalService(typeof(SVsRunningDocumentTable));
                if (rdt is null || !TryFindCookie(rdt, moniker, out uint cookie))
                {
                    return null;
                }

                if (rdt.GetDocumentInfo(cookie, out _, out _, out _, out _, out _, out _, out IntPtr data) != VSConstants.S_OK ||
                    data == IntPtr.Zero)
                {
                    return null;
                }

                return Marshal.GetObjectForIUnknown(data);
            }
            catch (Exception ex)
            {
                Log.Info($"Document data of {moniker} is not available: {ex.Message}");
                return null;
            }
        }

        // A PLC method is opened as a document of its own whose moniker
        // carries an "@" suffix, while the document in the table is the file.
        private static bool TryFindCookie(IVsRunningDocumentTable rdt, string moniker, out uint cookie)
        {
            cookie = 0;

            string wanted = WithoutPart(moniker);
            if (wanted is null)
            {
                return false;
            }

            if (rdt.GetRunningDocumentsEnum(out IEnumRunningDocuments documents) != VSConstants.S_OK || documents is null)
            {
                return false;
            }

            var cookies = new uint[1];
            while (documents.Next(1, cookies, out uint fetched) == VSConstants.S_OK && fetched == 1)
            {
                if (rdt.GetDocumentInfo(cookies[0], out _, out _, out _, out string candidate, out _, out _, out _) != VSConstants.S_OK)
                {
                    continue;
                }

                if (string.Equals(WithoutPart(candidate), wanted, StringComparison.OrdinalIgnoreCase))
                {
                    cookie = cookies[0];
                    return true;
                }
            }

            return false;
        }

        private static string WithoutPart(string moniker)
        {
            if (string.IsNullOrEmpty(moniker))
            {
                return null;
            }

            int at = moniker.LastIndexOf('@');
            return at < 0 ? moniker : moniker.Substring(0, at);
        }

        private static IEnumerable<object> Children(object node)
        {
            for (Type level = node.GetType(); level is not null && level != typeof(object); level = level.BaseType)
            {
                foreach (FieldInfo field in level.GetFields(Members | BindingFlags.DeclaredOnly))
                {
                    if (IsTraversable(field.FieldType))
                    {
                        object value = Read(() => field.GetValue(node));
                        if (value is not null)
                        {
                            yield return value;
                        }
                    }
                }

                foreach (PropertyInfo property in level.GetProperties(Members | BindingFlags.DeclaredOnly))
                {
                    if (property.GetIndexParameters().Length == 0 && IsTraversable(property.PropertyType))
                    {
                        object value = Read(() => property.GetValue(node));
                        if (value is not null)
                        {
                            yield return value;
                        }
                    }
                }
            }
        }

        private static bool IsTraversable(Type type)
        {
            if (!type.IsClass && !type.IsInterface)
            {
                return false;
            }

            string ns = type.Namespace;
            return ns is null || ns.StartsWith("Beckhoff.", StringComparison.Ordinal) ||
                ns.StartsWith("_3S.", StringComparison.Ordinal);
        }

        private static bool IsTwinCatObject(object node)
        {
            return node.GetType().Namespace?.StartsWith(TwinCatNamespace, StringComparison.Ordinal) == true;
        }

        // Calls a member and returns its result, or null when there is no
        // such member or it refused to run.
        private static object Call(object target, string name, params object[] arguments)
        {
            try
            {
                MethodInfo method = FindMethod(target.GetType(), name, arguments.Length);
                return method?.Invoke(target, arguments);
            }
            catch (Exception)
            {
                return null;
            }
        }

        // Calls a member for its effect. Returns false when there is no such
        // member or it threw.
        private static bool Invoke(object target, string name, params object[] arguments)
        {
            try
            {
                MethodInfo method = FindMethod(target.GetType(), name, arguments.Length);
                method?.Invoke(target, arguments);
                return method is not null;
            }
            catch (TargetInvocationException ex)
            {
                Log.Info($"{target.GetType().Name}.{name} failed: {ex.InnerException?.Message ?? ex.Message}");
                return false;
            }
            catch (Exception ex)
            {
                Log.Info($"{target.GetType().Name}.{name} failed: {ex.Message}");
                return false;
            }
        }

        private static MethodInfo FindMethod(Type type, string name, int count)
        {
            for (Type level = type; level is not null && level != typeof(object); level = level.BaseType)
            {
                foreach (MethodInfo method in level.GetMethods(Members | BindingFlags.DeclaredOnly))
                {
                    if (method.Name == name && method.GetParameters().Length == count)
                    {
                        return method;
                    }
                }
            }

            return null;
        }

        private static void Dispose(object editable)
        {
            try
            {
                (editable as IDisposable)?.Dispose();
            }
            catch (Exception ex)
            {
                Log.Info($"Releasing the editable TwinCAT text buffer failed: {ex.Message}");
            }
        }

        private static T Read<T>(Func<T> read)
        {
            try
            {
                return read();
            }
            catch (Exception)
            {
                // A member that cannot be read is not the one being looked for.
                return default;
            }
        }

        private static bool SameText(string left, string right)
        {
            return string.Equals(Normalize(left), Normalize(right), StringComparison.Ordinal);
        }

        private static string Normalize(string text)
        {
            return text?.Replace("\r\n", "\n").TrimEnd('\n');
        }

        private sealed class ReferenceComparer : IEqualityComparer<object>
        {
            internal static readonly ReferenceComparer Instance = new ReferenceComparer();

            public new bool Equals(object left, object right) => ReferenceEquals(left, right);

            public int GetHashCode(object value) => RuntimeHelpers.GetHashCode(value);
        }
    }
}