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
    private Forms.ToolStripMenuItem? _serverStartItem;
    private Forms.ToolStripMenuItem? _serverStopItem;
    private ServiceStatusSnapshot? _lastStatus;
    private TrayIconSet? _trayIcons;
    private System.Windows.Media.ImageSource? _windowIconBase;
    private System.Windows.Media.ImageSource? _windowIconEnabled;
    private System.Windows.Media.ImageSource? _windowIconBusy;
    private DispatcherTimer? _statusTimer;
    private string? _lastStatusKey;
    private TrayIconState? _lastTrayIconState;

    public App()
    {
        ShutdownMode = ShutdownMode.OnExplicitShutdown;
        Startup += OnStartup;
        Exit += OnExit;
    }

    private void OnStartup(object? sender, StartupEventArgs e)
    {
        Log("xp2p-ui starting.");
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
        Log("xp2p-ui exiting.");
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
        _clientMenu.DropDownItems.Add(_clientStartItem);
        _clientMenu.DropDownItems.Add(_clientStopItem);
        _clientMenu.DropDownItems.Add(new Forms.ToolStripMenuItem("Install", null, (_, _) => ShowWindow("Client install.", TabKey.ClientInstall)));
        _clientMenu.DropDownItems.Add(new Forms.ToolStripMenuItem("Deploy", null, (_, _) => ShowWindow("Client deploy.", TabKey.ClientDeploy)));

        _serverMenu = new Forms.ToolStripMenuItem("Server: Unknown");
        _serverStartItem = new Forms.ToolStripMenuItem("Start", null, (_, _) => StartServiceAsync(ServiceNames.Server));
        _serverStopItem = new Forms.ToolStripMenuItem("Stop", null, (_, _) => StopServiceAsync(ServiceNames.Server));
        _serverMenu.DropDownItems.Add(_serverStartItem);
        _serverMenu.DropDownItems.Add(_serverStopItem);
        _serverMenu.DropDownItems.Add(new Forms.ToolStripMenuItem("Install", null, (_, _) => ShowWindow("Server install.", TabKey.ServerInstall)));
        _serverMenu.DropDownItems.Add(new Forms.ToolStripMenuItem("Deploy", null, (_, _) => ShowWindow("Server deploy.", TabKey.ServerDeploy)));

        var openLogs = new Forms.ToolStripMenuItem("Open logs", null, (_, _) => OpenLogs());
        var quit = new Forms.ToolStripMenuItem("Quit", null, (_, _) => Shutdown());

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

    private void RefreshServiceStatus()
    {
        if (_serviceManager is null)
        {
            return;
        }
        var snapshot = _serviceManager.GetSnapshot();
        _lastStatus = snapshot;
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
            _trayIcon.Text = BuildTrayTooltip(snapshot);
        }
        var statusKey = BuildStatusKey(snapshot, _serviceManager?.IsBusy ?? false);
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

    private static bool IsServiceRunning(string status)
    {
        return string.Equals(status, "Running", StringComparison.OrdinalIgnoreCase);
    }

    private void UpdateTrayServiceButtons(ServiceStatusSnapshot snapshot)
    {
        UpdateTrayButtonsForStatus(snapshot.ClientStatus, _clientStartItem, _clientStopItem);
        UpdateTrayButtonsForStatus(snapshot.ServerStatus, _serverStartItem, _serverStopItem);
    }

    private static void UpdateTrayButtonsForStatus(string status, Forms.ToolStripMenuItem? start, Forms.ToolStripMenuItem? stop)
    {
        if (start is null || stop is null)
        {
            return;
        }
        if (IsServiceRunning(status))
        {
            start.Enabled = false;
            stop.Enabled = true;
            return;
        }
        if (string.Equals(status, "Stopped", StringComparison.OrdinalIgnoreCase))
        {
            start.Enabled = true;
            stop.Enabled = false;
            return;
        }
        if (IsServicePending(status))
        {
            start.Enabled = false;
            stop.Enabled = false;
            return;
        }
        start.Enabled = true;
        stop.Enabled = true;
    }

    private static bool IsServicePending(string status)
    {
        return string.Equals(status, "StartPending", StringComparison.OrdinalIgnoreCase) ||
            string.Equals(status, "StopPending", StringComparison.OrdinalIgnoreCase) ||
            string.Equals(status, "PausePending", StringComparison.OrdinalIgnoreCase) ||
            string.Equals(status, "ContinuePending", StringComparison.OrdinalIgnoreCase);
    }

    private static string BuildTrayTooltip(ServiceStatusSnapshot snapshot)
    {
        var text = $"Client: {snapshot.ClientStatus} | Server: {snapshot.ServerStatus}";
        return text.Length <= 63 ? text : text.Substring(0, 63);
    }

    private static string GetLogPath()
    {
        var programData = Environment.GetFolderPath(Environment.SpecialFolder.CommonApplicationData);
        if (string.IsNullOrWhiteSpace(programData))
        {
            programData = @"C:\ProgramData";
        }
        return Path.Combine(programData, "xp2p", "logs", "xp2p-ui.log");
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
        var snapshot = _lastStatus;
        if (snapshot is not null &&
            (IsServiceRunning(snapshot.ClientStatus) || IsServiceRunning(snapshot.ServerStatus)))
        {
            return TrayIconState.Enabled;
        }
        return TrayIconState.Disabled;
    }

    private static string BuildStatusKey(ServiceStatusSnapshot snapshot, bool busy)
    {
        return $"{snapshot.ClientStatus}|{snapshot.ServerStatus}|{busy}";
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

}
