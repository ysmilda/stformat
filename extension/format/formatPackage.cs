global using Community.VisualStudio.Toolkit;
global using Microsoft.VisualStudio.Shell;
global using System;
global using Task = System.Threading.Tasks.Task;
using System.Runtime.InteropServices;
using System.Threading;

namespace format
{
    [PackageRegistration(UseManagedResourcesOnly = true, AllowsBackgroundLoading = true)]
    [InstalledProductRegistration(Vsix.Name, Vsix.Description, Vsix.Version)]
    [ProvideMenuResource("Menus.ctmenu", 1)]
    [ProvideAutoLoad("adfc4e64-0397-11d1-9f4e-00a0c911004f", PackageAutoLoadFlags.BackgroundLoad)]
    [ProvideAutoLoad("f1536ef8-92ec-443c-9ed7-fdadf150da82", PackageAutoLoadFlags.BackgroundLoad)]
    [Guid(PackageGuids.formatString)]
    public sealed class formatPackage : ToolkitPackage
    {
        protected override async Task InitializeAsync(CancellationToken cancellationToken, IProgress<ServiceProgressData> progress)
        {
            Log.Info("Package initializing");

            await this.RegisterCommandsAsync();
            Log.Info("Commands registered");

            await ThreadHelper.JoinableTaskFactory.SwitchToMainThreadAsync(cancellationToken);
            Format.RegisterOnSave();
            Log.Info("Save handler registered");
        }
    }
}