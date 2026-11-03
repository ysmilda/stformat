using System.ComponentModel;
using System.Runtime.InteropServices;

namespace format
{
    internal partial class OptionsProvider
    {
        // Shown under Tools > Options > StFormat > General.
        [ComVisible(true)]
        public class GeneralOptions : BaseOptionPage<General> { }
    }

    public class General : BaseOptionModel<General>
    {
        [Category("Formatting")]
        [DisplayName("Format on save")]
        [Description("Format Structured Text and TwinCAT PLC objects every time they are saved.")]
        [DefaultValue(true)]
        public bool FormatOnSave { get; set; } = true;
    }
}
