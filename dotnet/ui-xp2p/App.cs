using System;
using System.Diagnostics;
using System.IO;
using System.Windows;
using System.Windows.Threading;
using Application = System.Windows.Application;
using Forms = System.Windows.Forms;

namespace Xp2pUi;

internal sealed class App : Application
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
    private ServiceStatusSnapshot? _lastStatus;
    private TrayIconSet? _trayIcons;
    private System.Windows.Media.ImageSource? _windowIconBase;
    private System.Windows.Media.ImageSource? _windowIconEnabled;
    private System.Windows.Media.ImageSource? _windowIconBusy;
    private DispatcherTimer? _statusTimer;
    private string? _lastStatusKey;
    private TrayIconState? _lastTrayIconState;
    private ClientRuntimeView? _clientRuntimeView;

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

        var appIcon = GetAppIcon();
        _trayIcons = TrayIconLoader.Load(appIcon, Log);
        _windowIconBase = TrayIconLoader.CreateIconSource(_trayIcons.Base);
        _windowIconEnabled = TrayIconLoader.CreateIconSource(_trayIcons.Enabled);
        _windowIconBusy = TrayIconLoader.CreateIconSource(_trayIcons.Busy);
        _window = new MainWindow(_backend, _serviceManager, _windowIconBase);
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

    private Forms.ContextMenuStrip BuildMenu()
    {
        _clientMenu = new Forms.ToolStripMenuItem("Client: Unknown");
        _clientStartItem = new Forms.ToolStripMenuItem("Start", null, (_, _) => StartServiceAsync(ServiceNames.Client));
        _clientStopItem = new Forms.ToolStripMenuItem("Stop", null, (_, _) => StopServiceAsync(ServiceNames.Client));
        _clientRestartItem = new Forms.ToolStripMenuItem("Restart", null, (_, _) => RestartServiceAsync(ServiceNames.Client));
        _clientMenu.DropDownItems.Add(_clientStartItem);
        _clientMenu.DropDownItems.Add(_clientStopItem);
        _clientMenu.DropDownItems.Add(_clientRestartItem);
        _serverMenu = new Forms.ToolStripMenuItem("Server: Unknown");
        _serverStartItem = new Forms.ToolStripMenuItem("Start", null, (_, _) => StartServiceAsync(ServiceNames.Server));
        _serverStopItem = new Forms.ToolStripMenuItem("Stop", null, (_, _) => StopServiceAsync(ServiceNames.Server));
        _serverRestartItem = new Forms.ToolStripMenuItem("Restart", null, (_, _) => RestartServiceAsync(ServiceNames.Server));
        _serverMenu.DropDownItems.Add(_serverStartItem);
        _serverMenu.DropDownItems.Add(_serverStopItem);
        _serverMenu.DropDownItems.Add(_serverRestartItem);

        var openLogs = new Forms.ToolStripMenuItem("Open logs", null, (_, _) => OpenLogs());
        var quit = new Forms.ToolStripMenuItem("Quit", null, (_, _) => RequestShutdown());

        var menu = new Forms.ContextMenuStrip();
        menu.Opening += (_, _) => RefreshServiceStatus();
        menu.Items.Add(_clientMenu);
        menu.Items.Add(_serverMenu);
        menu.Items.Add(new Forms.ToolStripSeparator());
        menu.Items.Add(openLogs);
        menu.Items.Add(new Forms.ToolStripSeparator());
        menu.Items.Add(quit);
        return menu;
    }

    private async void RequestShutdown()
    {
        if (ShouldPromptStopServices())
        {
            var result = Dispatcher.Invoke(() => System.Windows.MessageBox.Show(
                _window,
                "Stop all services?",
                "xp2p",
                MessageBoxButton.YesNo,
                MessageBoxImage.Question));
            if (result == MessageBoxResult.Yes)
            {
                await StopServicesAsync();
            }
        }
        Shutdown();
    }

    private async System.Threading.Tasks.Task StopServicesAsync()
    {
        if (_serviceManager is null)
        {
            return;
        }
        var stopClient = _serviceManager.StopServiceAsync(ServiceNames.Client);
        var stopServer = _serviceManager.StopServiceAsync(ServiceNames.Server);
        await System.Threading.Tasks.Task.WhenAll(stopClient, stopServer);
    }

    private void ShowWindow(string message, TabKey tab)
    {
        if (_window is null)
        {
            return;
        }
        Dispatcher.Invoke(() =>
        {
            _window.SetStatus(message);
            _window.SelectTab(tab);
            _window.Show();
            _window.Activate();
        });
    }

    private void OnServiceActivityChanged(object? sender, bool busy)
    {
        if (_trayIcon is null)
        {
            return;
        }
        if (busy && _trayIcons?.Busy is not null)
        {
            _trayIcon.Icon = _trayIcons.Busy;
            UpdateWindowIcon(_windowIconBusy);
            return;
        }
        UpdateTrayIconFromStatus();
    }

    private void OnServiceStatusChanged(object? sender, ServiceStatusSnapshot snapshot)
    {
        _lastStatus = snapshot;
        RefreshClientRuntimeStatus(snapshot);
        Log($"service status changed: client={snapshot.ClientStatus} server={snapshot.ServerStatus} busy={_serviceManager?.IsBusy}");
        UpdateTrayStatusLabels(snapshot);
        UpdateTrayIconFromStatus();
    }

    private void OpenLogs()
    {
        var logPath = GetLogPath();
        if (!File.Exists(logPath))
        {
            SetStatus($"Log file not found: {logPath}");
            return;
        }

        try
        {
            Process.Start(new ProcessStartInfo
            {
                FileName = "explorer.exe",
                Arguments = $"/select,\"{logPath}\"",
                UseShellExecute = true
            });
            SetStatus("Log file opened.");
        }
        catch (Exception ex)
        {
            SetStatus($"Open log failed: {ex.Message}");
        }
    }

    private async void StartServiceAsync(string name)
    {
        if (_serviceManager is null)
        {
            return;
        }
        if (_trayIcon is not null && _trayIcons?.Busy is not null)
        {
            _trayIcon.Icon = _trayIcons.Busy;
            UpdateWindowIcon(_windowIconBusy);
        }
        var result = await _serviceManager.StartServiceAsync(name);
        SetStatus(result);
    }

    private async void StopServiceAsync(string name)
    {
        if (_serviceManager is null)
        {
            return;
        }
        if (_trayIcon is not null && _trayIcons?.Busy is not null)
        {
            _trayIcon.Icon = _trayIcons.Busy;
            UpdateWindowIcon(_windowIconBusy);
        }
        var result = await _serviceManager.StopServiceAsync(name);
        SetStatus(result);
    }

    private async void RestartServiceAsync(string name)
    {
        if (_serviceManager is null)
        {
            return;
        }
        if (_trayIcon is not null && _trayIcons?.Busy is not null)
        {
            _trayIcon.Icon = _trayIcons.Busy;
            UpdateWindowIcon(_windowIconBusy);
        }
        var result = await _serviceManager.RestartServiceAsync(name);
        SetStatus(result);
    }

    private void RefreshServiceStatus()
    {
        if (_serviceManager is null)
        {
            return;
        }
        var snapshot = _serviceManager.GetSnapshot();
        _lastStatus = snapshot;
        RefreshClientRuntimeStatus(snapshot);
        UpdateTrayStatusLabels(snapshot);
        UpdateTrayIconFromStatus();
    }

    private void SetStatus(string message)
    {
        if (_window is null)
        {
            return;
        }
        Dispatcher.Invoke(() => _window.SetStatus(message));
    }

    private void UpdateTrayStatusLabels(ServiceStatusSnapshot snapshot)
    {
        if (_clientMenu is not null)
        {
            _clientMenu.Text = $"Client: {snapshot.ClientStatus}";
        }
        if (_serverMenu is not null)
        {
            _serverMenu.Text = $"Server: {snapshot.ServerStatus}";
        }
        UpdateTrayServiceButtons(snapshot);
        if (_trayIcon is not null)
        {
            _trayIcon.Text = UiLogic.BuildTrayTooltip(snapshot, _clientRuntimeView);
        }
        var statusKey = BuildStatusKey(snapshot, _serviceManager?.IsBusy ?? false, _clientRuntimeView);
        if (!string.Equals(_lastStatusKey, statusKey, StringComparison.Ordinal))
        {
            _lastStatusKey = statusKey;
            Log($"tray status: client={snapshot.ClientStatus} server={snapshot.ServerStatus} busy={_serviceManager?.IsBusy}");
        }
    }

    private void UpdateTrayIconFromStatus()
    {
        if (_trayIcon is null || _trayIcons is null)
        {
            return;
        }
        var desired = GetTrayIconState();
        if (_lastTrayIconState == desired)
        {
            return;
        }
        _lastTrayIconState = desired;
        var snapshot = _lastStatus;
        switch (desired)
        {
            case TrayIconState.Busy:
                _trayIcon.Icon = _trayIcons.Busy;
                Log("tray icon: busy");
                UpdateWindowIcon(_windowIconBusy);
                break;
            case TrayIconState.Enabled:
                _trayIcon.Icon = _trayIcons.Enabled;
                Log("tray icon: enabled");
                UpdateWindowIcon(_windowIconEnabled);
                break;
            default:
                _trayIcon.Icon = _trayIcons.Base;
                _trayIcon.Text = "xp2p";
                Log(snapshot is null ? "tray icon: disabled (no snapshot)" : "tray icon: disabled");
                UpdateWindowIcon(_windowIconBase);
                break;
        }
    }

    private void UpdateTrayServiceButtons(ServiceStatusSnapshot snapshot)
    {
        UpdateTrayButtonsForStatus(snapshot.ClientStatus, _clientStartItem, _clientStopItem, _clientRestartItem);
        UpdateTrayButtonsForStatus(snapshot.ServerStatus, _serverStartItem, _serverStopItem, _serverRestartItem);
    }

    private static void UpdateTrayButtonsForStatus(string status, Forms.ToolStripMenuItem? start, Forms.ToolStripMenuItem? stop, Forms.ToolStripMenuItem? restart)
    {
        if (start is null || stop is null)
        {
            return;
        }
        var state = UiLogic.GetServiceButtonState(status);
        start.Enabled = state.StartEnabled;
        stop.Enabled = state.StopEnabled;
        if (restart is not null)
        {
            restart.Enabled = UiLogic.IsRestartEnabled(status);
        }
    }

    private bool ShouldPromptStopServices()
    {
        if (_serviceManager is null)
        {
            return false;
        }
        return UiLogic.ShouldPromptStopServices(_serviceManager.GetSnapshot());
    }

    private static string GetLogPath()
    {
        var programData = Environment.GetFolderPath(Environment.SpecialFolder.CommonApplicationData);
        if (string.IsNullOrWhiteSpace(programData))
        {
            programData = @"C:\ProgramData";
        }
        return Path.Combine(programData, "xp2p", "logs", "ui-xp2p.log");
    }

    private void Log(string message)
    {
        try
        {
            var logPath = GetLogPath();
            var logDir = Path.GetDirectoryName(logPath);
            if (!string.IsNullOrWhiteSpace(logDir))
            {
                Directory.CreateDirectory(logDir);
            }
            var line = $"{DateTime.UtcNow:O} INFO xp2p: {message}";
            File.AppendAllText(logPath, line + Environment.NewLine);
        }
        catch
        {
            // Avoid crashing on log failures.
        }
    }

    private void LogResourcesHint()
    {
        try
        {
            var assembly = GetType().Assembly;
            foreach (var name in assembly.GetManifestResourceNames())
            {
                if (!name.EndsWith(".png", StringComparison.OrdinalIgnoreCase))
                {
                    continue;
                }
                Log($"resource: {name}");
            }
        }
        catch
        {
            // Ignore resource log failures.
        }
    }

    private static System.Drawing.Icon GetAppIcon()
    {
        var exePath = Process.GetCurrentProcess().MainModule?.FileName;
        if (!string.IsNullOrWhiteSpace(exePath))
        {
            try
            {
                var icon = System.Drawing.Icon.ExtractAssociatedIcon(exePath);
                if (icon is not null)
                {
                    return icon;
                }
            }
            catch
            {
                // Fall back to the default icon.
            }
        }
        return System.Drawing.SystemIcons.Application;
    }

    private void StartStatusTimer()
    {
        _statusTimer = new DispatcherTimer
        {
            Interval = GetStatusPollInterval()
        };
        _statusTimer.Tick += (_, _) => RefreshServiceStatus();
        _statusTimer.Start();
    }

    private static TimeSpan GetStatusPollInterval()
    {
        var raw = Environment.GetEnvironmentVariable("XP2P_UI_STATUS_POLL_SECONDS");
        if (int.TryParse(raw, out var seconds) && seconds > 0 && seconds <= 60)
        {
            return TimeSpan.FromSeconds(seconds);
        }
        return TimeSpan.FromSeconds(5);
    }

    private TrayIconState GetTrayIconState()
    {
        if (_serviceManager is not null && _serviceManager.IsBusy)
        {
            return TrayIconState.Busy;
        }
        if (_clientRuntimeView is not null)
        {
            return _clientRuntimeView.Status switch
            {
                ClientRuntimeStatus.Ready => TrayIconState.Enabled,
                ClientRuntimeStatus.Pending => TrayIconState.Busy,
                _ => TrayIconState.Disabled
            };
        }
        var snapshot = _lastStatus;
        if (snapshot is not null &&
            (UiLogic.IsServiceRunning(snapshot.ClientStatus) || UiLogic.IsServiceRunning(snapshot.ServerStatus)))
        {
            return TrayIconState.Enabled;
        }
        return TrayIconState.Disabled;
    }

    private static string BuildStatusKey(ServiceStatusSnapshot snapshot, bool busy, ClientRuntimeView? runtime)
    {
        return $"{snapshot.ClientStatus}|{snapshot.ServerStatus}|{busy}|{runtime?.Status}|{runtime?.Summary}";
    }

    private void UpdateWindowIcon(System.Windows.Media.ImageSource? icon)
    {
        if (_window is null || icon is null)
        {
            return;
        }
        Dispatcher.Invoke(() => _window.Icon = icon);
    }

    private enum TrayIconState
    {
        Disabled,
        Enabled,
        Busy
    }

    private void RefreshClientRuntimeStatus(ServiceStatusSnapshot snapshot)
    {
        if (_window is null)
        {
            return;
        }
        var statePath = GetClientStatePath();
        var state = ClientStateReader.TryLoad(statePath);
        _clientRuntimeView = UiLogic.BuildClientRuntimeView(snapshot.ClientStatus, state);
        var view = _clientRuntimeView;
        if (view is null)
        {
            return;
        }
        var message = view.Detail;
        if (!string.IsNullOrWhiteSpace(view.LastError))
        {
            message = $"{message} | Error: {view.LastError}";
        }
        Dispatcher.Invoke(() => _window.SetClientRuntimeStatus(message));
    }

    private static string GetClientStatePath()
    {
        var programData = Environment.GetFolderPath(Environment.SpecialFolder.CommonApplicationData);
        if (string.IsNullOrWhiteSpace(programData))
        {
            programData = @"C:\ProgramData";
        }
        return Path.Combine(programData, "xp2p", "xp2p-client.state.json");
    }

}
