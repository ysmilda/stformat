using System.Collections.Generic;
using System.ComponentModel.Composition;
using Microsoft.VisualStudio.Text.Editor;
using Microsoft.VisualStudio.Utilities;

namespace format
{
    // Keeps track of the open text views. TwinCAT editors host the
    // declaration and the implementation of a PLC object in text views that
    // the running document table does not list; editing their buffers instead
    // of setting the automation properties keeps formatting undoable.
    [Export(typeof(IWpfTextViewCreationListener))]
    [ContentType("any")]
    [TextViewRole(PredefinedTextViewRoles.Editable)]
    internal sealed class TextViewTracker : IWpfTextViewCreationListener
    {
        // Views are created and closed on the UI thread, which is also the
        // only thread that reads the list.
        private static readonly List<IWpfTextView> Views = new List<IWpfTextView>();

        internal static IReadOnlyList<IWpfTextView> OpenViews => Views.ToArray();

        public void TextViewCreated(IWpfTextView textView)
        {
            Views.Add(textView);
            textView.Closed += (sender, e) => Views.Remove(textView);
        }
    }
}
