using System;
using System.Windows;
using System.Windows.Threading;
using Application = System.Windows.Application;
using Forms = System.Windows.Forms;

namespace Xp2pUi;

internal sealed partial class App : Application
{
    private Forms.NotifyIcon? _trayIcon;
    private MainWindow? _window;
    private ServiceManager? _serviceManager;
    private IBackend? _backend;
    private Forms.ToolStripMenuItem? _clientMenu;
    private Forms.ToolStripMenuItem? _serverMenu;
    private Forms.ToolStripMenuItem? _clientStartItem;
    private Forms.ToolStripMenuItem? _clientStopItem;
    private Forms.ToolStripMenuItem? _clientRestartItem;
    private Forms.ToolStripMenuItem? _serverStartItem;
    private Forms.ToolStripMenuItem? _serverStopItem;
    private Forms.ToolStripMenuItem? _serverRestartItem;
    private Forms.ToolStripMenuItem? _clientModeMenu;
    private Forms.ToolStripMenuItem? _clientModeProxyItem;
    private Forms.ToolStripMenuItem? _clientModeSplitItem;
    private Forms.ToolStripMenuItem? _clientModeFullItem;
    private Forms.ToolStripMenuItem? _serverModeMenu;
    private Forms.ToolStripMenuItem? _serverModeProxyItem;
    private Forms.ToolStripMenuItem? _serverModeTunItem;
    private Forms.ToolStripMenuItem? _serverModeUnsupportedItem;
    private ServiceStatusSnapshot? _lastStatus;
    private TrayIconSet? _trayIcons;
    private System.Windows.Media.ImageSource? _windowIconBase;
    private System.Windows.Media.ImageSource? _windowIconEnabled;
    private System.Windows.Media.ImageSource? _windowIconBusy;
    private DispatcherTimer? _statusTimer;
    private string? _lastStatusKey;
    private TrayIconState? _lastTrayIconState;
    private ClientRuntimeView? _clientRuntimeView;
    private ModeManager? _modeManager;
    private ClientMode? _pendingClientMode;
    private ServerMode? _pendingServerMode;

    public App()
    {
        ShutdownMode = ShutdownMode.OnExplicitShutdown;
        Startup += OnStartup;
        Exit += OnExit;
    }

    private void OnStartup(object? sender, StartupEventArgs e)
    {
        Log("ui-xp2p starting.");
        Log($"base dir: {AppContext.BaseDirectory}");
        Log($"assembly name: {GetType().Assembly.GetName().Name}");
        LogResourcesHint();
        Resources.MergedDictionaries.Add(new ModernWpf.Controls.XamlControlsResources());
        Resources.MergedDictionaries.Add(new ModernWpf.ThemeResources());
        _backend = BackendFactory.Create();
        _serviceManager = new ServiceManager();
        _serviceManager.ActivityChanged += OnServiceActivityChanged;
        _serviceManager.StatusChanged += OnServiceStatusChanged;
        _modeManager = new ModeManager();

        var appIcon = GetAppIcon();
        _trayIcons = TrayIconLoader.Load(appIcon, Log);
        _windowIconBase = TrayIconLoader.CreateIconSource(_trayIcons.Base);
        _windowIconEnabled = TrayIconLoader.CreateIconSource(_trayIcons.Enabled);
        _windowIconBusy = TrayIconLoader.CreateIconSource(_trayIcons.Busy);
        _window = new MainWindow(_backend, _serviceManager, _windowIconBase);
        _window.ClientModeRequested += (_, mode) => RequestClientMode(mode);
        _window.ServerModeRequested += (_, mode) => RequestServerMode(mode);
        _window.Hide();

        _trayIcon = new Forms.NotifyIcon
        {
            Icon = _trayIcons.Base,
            Text = "xp2p",
            Visible = true,
            ContextMenuStrip = BuildMenu()
        };
        _trayIcon.DoubleClick += (_, _) => ShowWindow("Ready.", TabKey.Status);
        RefreshServiceStatus();
        StartStatusTimer();
    }

    private void OnExit(object? sender, ExitEventArgs e)
    {
        Log("ui-xp2p exiting.");
        if (_statusTimer is not null)
        {
            _statusTimer.Stop();
        }
        if (_trayIcon is not null)
        {
            _trayIcon.Visible = false;
            _trayIcon.Dispose();
        }
        _trayIcons?.Dispose();
    }
}
