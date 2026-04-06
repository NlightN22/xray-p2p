using System;
using System.Windows.Media;
using Forms = System.Windows.Forms;

namespace Xp2pUi;

internal sealed partial class App
{
    private Forms.ContextMenuStrip BuildMenu()
    {
        _clientMenu = new Forms.ToolStripMenuItem("Client: Unknown");
        _clientStartItem = new Forms.ToolStripMenuItem("Start", null, (_, _) => StartServiceAsync(ServiceNames.Client));
        _clientStopItem = new Forms.ToolStripMenuItem("Stop", null, (_, _) => StopServiceAsync(ServiceNames.Client));
        _clientRestartItem = new Forms.ToolStripMenuItem("Restart", null, (_, _) => RestartServiceAsync(ServiceNames.Client));
        _clientMenu.DropDownItems.Add(_clientStartItem);
        _clientMenu.DropDownItems.Add(_clientStopItem);
        _clientMenu.DropDownItems.Add(_clientRestartItem);
        _clientMenu.DropDownItems.Add(new Forms.ToolStripSeparator());
        _clientModeMenu = new Forms.ToolStripMenuItem("Mode: Unknown");
        _clientModeProxyItem = new Forms.ToolStripMenuItem("Proxy", null, (_, _) => RequestClientMode(ClientMode.Proxy));
        _clientModeSplitItem = new Forms.ToolStripMenuItem("Tun Split", null, (_, _) => RequestClientMode(ClientMode.TunSplit));
        _clientModeFullItem = new Forms.ToolStripMenuItem("Tun Full", null, (_, _) => RequestClientMode(ClientMode.TunFull));
        _clientModeMenu.DropDownItems.Add(_clientModeProxyItem);
        _clientModeMenu.DropDownItems.Add(_clientModeSplitItem);
        _clientModeMenu.DropDownItems.Add(_clientModeFullItem);
        _clientMenu.DropDownItems.Add(_clientModeMenu);
        _serverMenu = new Forms.ToolStripMenuItem("Server: Unknown");
        _serverStartItem = new Forms.ToolStripMenuItem("Start", null, (_, _) => StartServiceAsync(ServiceNames.Server));
        _serverStopItem = new Forms.ToolStripMenuItem("Stop", null, (_, _) => StopServiceAsync(ServiceNames.Server));
        _serverRestartItem = new Forms.ToolStripMenuItem("Restart", null, (_, _) => RestartServiceAsync(ServiceNames.Server));
        _serverMenu.DropDownItems.Add(_serverStartItem);
        _serverMenu.DropDownItems.Add(_serverStopItem);
        _serverMenu.DropDownItems.Add(_serverRestartItem);
        _serverMenu.DropDownItems.Add(new Forms.ToolStripSeparator());
        _serverModeMenu = new Forms.ToolStripMenuItem("Mode: Unknown");
        _serverModeProxyItem = new Forms.ToolStripMenuItem("Proxy", null, (_, _) => RequestServerMode(ServerMode.Proxy));
        _serverModeTunItem = new Forms.ToolStripMenuItem("Tun", null, (_, _) => RequestServerMode(ServerMode.Tun));
        _serverModeUnsupportedItem = new Forms.ToolStripMenuItem("Split/Full modes are not supported on server.") { Enabled = false };
        _serverModeMenu.DropDownItems.Add(_serverModeProxyItem);
        _serverModeMenu.DropDownItems.Add(_serverModeTunItem);
        _serverModeMenu.DropDownItems.Add(new Forms.ToolStripSeparator());
        _serverModeMenu.DropDownItems.Add(_serverModeUnsupportedItem);
        _serverMenu.DropDownItems.Add(_serverModeMenu);

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

    private void UpdateWindowIcon(ImageSource? icon)
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
