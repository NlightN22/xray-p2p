using System;
using System.Diagnostics;
using System.IO;
using System.Windows;
using Application = System.Windows.Application;
using Forms = System.Windows.Forms;

namespace Xp2pUi;

internal sealed class App : Application
{
    private Forms.NotifyIcon? _trayIcon;
    private MainWindow? _window;
    private ServiceManager? _serviceManager;
    private IBackend? _backend;
    private Forms.ToolStripMenuItem? _clientStatusItem;
    private Forms.ToolStripMenuItem? _serverStatusItem;
    private ServiceStatusSnapshot? _lastStatus;
    private TrayIconSet? _trayIcons;

    public App()
    {
        ShutdownMode = ShutdownMode.OnExplicitShutdown;
        Startup += OnStartup;
        Exit += OnExit;
    }

    private void OnStartup(object? sender, StartupEventArgs e)
    {
        Log("xp2p-ui starting.");
        _backend = BackendFactory.Create();
        _serviceManager = new ServiceManager();
        _serviceManager.ActivityChanged += OnServiceActivityChanged;
        _serviceManager.StatusChanged += OnServiceStatusChanged;

        var appIcon = GetAppIcon();
        _trayIcons = TrayIconLoader.Load(appIcon);
        _window = new MainWindow(_backend, _serviceManager, TrayIconLoader.CreateIconSource(appIcon));
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
    }

    private void OnExit(object? sender, ExitEventArgs e)
    {
        Log("xp2p-ui exiting.");
        if (_trayIcon is not null)
        {
            _trayIcon.Visible = false;
            _trayIcon.Dispose();
        }
        _trayIcons?.Dispose();
    }

    private Forms.ContextMenuStrip BuildMenu()
    {
        _clientStatusItem = new Forms.ToolStripMenuItem("Client: Unknown") { Enabled = false };
        _serverStatusItem = new Forms.ToolStripMenuItem("Server: Unknown") { Enabled = false };

        var clientMenu = new Forms.ToolStripMenuItem("Client");
        clientMenu.DropDownItems.Add(new Forms.ToolStripMenuItem("Start", null, (_, _) => StartService(ServiceNames.Client)));
        clientMenu.DropDownItems.Add(new Forms.ToolStripMenuItem("Stop", null, (_, _) => StopService(ServiceNames.Client)));
        clientMenu.DropDownItems.Add(new Forms.ToolStripMenuItem("Install", null, (_, _) => ShowWindow("Client install.", TabKey.ClientInstall)));
        clientMenu.DropDownItems.Add(new Forms.ToolStripMenuItem("Deploy", null, (_, _) => ShowWindow("Client deploy.", TabKey.ClientDeploy)));

        var serverMenu = new Forms.ToolStripMenuItem("Server");
        serverMenu.DropDownItems.Add(new Forms.ToolStripMenuItem("Start", null, (_, _) => StartService(ServiceNames.Server)));
        serverMenu.DropDownItems.Add(new Forms.ToolStripMenuItem("Stop", null, (_, _) => StopService(ServiceNames.Server)));
        serverMenu.DropDownItems.Add(new Forms.ToolStripMenuItem("Install", null, (_, _) => ShowWindow("Server install.", TabKey.ServerInstall)));
        serverMenu.DropDownItems.Add(new Forms.ToolStripMenuItem("Deploy", null, (_, _) => ShowWindow("Server deploy.", TabKey.ServerDeploy)));

        var openLogs = new Forms.ToolStripMenuItem("Open logs", null, (_, _) => OpenLogs());
        var quit = new Forms.ToolStripMenuItem("Quit", null, (_, _) => Shutdown());

        var menu = new Forms.ContextMenuStrip();
        menu.Opening += (_, _) => RefreshServiceStatus();
        menu.Items.Add(_clientStatusItem);
        menu.Items.Add(_serverStatusItem);
        menu.Items.Add(new Forms.ToolStripSeparator());
        menu.Items.Add(clientMenu);
        menu.Items.Add(serverMenu);
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
            return;
        }
        UpdateTrayIconFromStatus();
    }

    private void OnServiceStatusChanged(object? sender, ServiceStatusSnapshot snapshot)
    {
        _lastStatus = snapshot;
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

    private void StartService(string name)
    {
        if (_serviceManager is null)
        {
            return;
        }
        var result = _serviceManager.StartService(name);
        SetStatus(result);
        RefreshServiceStatus();
    }

    private void StopService(string name)
    {
        if (_serviceManager is null)
        {
            return;
        }
        var result = _serviceManager.StopService(name);
        SetStatus(result);
        RefreshServiceStatus();
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
        if (_clientStatusItem is not null)
        {
            _clientStatusItem.Text = $"Client: {snapshot.ClientStatus}";
        }
        if (_serverStatusItem is not null)
        {
            _serverStatusItem.Text = $"Server: {snapshot.ServerStatus}";
        }
    }

    private void UpdateTrayIconFromStatus()
    {
        if (_trayIcon is null || _trayIcons is null)
        {
            return;
        }
        var snapshot = _lastStatus;
        if (snapshot is null)
        {
            _trayIcon.Icon = _trayIcons.Base;
            return;
        }
        if (IsServiceRunning(snapshot.ClientStatus) || IsServiceRunning(snapshot.ServerStatus))
        {
            _trayIcon.Icon = _trayIcons.Enabled;
            return;
        }
        _trayIcon.Icon = _trayIcons.Base;
    }

    private static bool IsServiceRunning(string status)
    {
        return string.Equals(status, "Running", StringComparison.OrdinalIgnoreCase);
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

}
