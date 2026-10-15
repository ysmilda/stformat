using Microsoft.VisualStudio.Shell;
using Microsoft.VisualStudio.Text;
using Microsoft.VisualStudio.Text.Editor;

namespace format
{
    // Reaches the text buffer of the editor that shows a document. Only
    // documents opened by the Visual Studio text editor have one; a document
    // shown by a custom editor (a TwinCAT PLC object, for example) has no
    // view here at all and no buffer to edit.
    internal static class EditorBuffer
    {
        internal static ITextBuffer TryGet(IWpfTextView view)
        {
            return view?.TextBuffer;
        }

        // Replaces the whole buffer content in a single edit. Visual Studio
        // records that edit as one undo step, so Ctrl-Z brings back exactly
        // the text the buffer held before formatting, and the rest of the
        // undo history of the document survives.
        internal static void ReplaceAll(ITextBuffer buffer, string text)
        {
            ThreadHelper.ThrowIfNotOnUIThread();

            using (ITextEdit edit = buffer.CreateEdit())
            {
                edit.Replace(0, buffer.CurrentSnapshot.Length, text);
                edit.Apply();
            }
        }
    }
}