namespace Xp2pUi;

internal sealed partial class App
{
    private void UpdateTrayClientModeMenu(ClientMode? mode, string label, bool pending)
    {
        if (_clientModeMenu is not null)
        {
            _clientModeMenu.Text = $"Mode: {label}";
        }
        if (_clientModeProxyItem is null || _clientModeSplitItem is null || _clientModeFullItem is null)
        {
            return;
        }
        var enabled = !pending;
        _clientModeProxyItem.Enabled = enabled && mode != ClientMode.Proxy;
        _clientModeSplitItem.Enabled = enabled && mode != ClientMode.TunSplit;
        _clientModeFullItem.Enabled = enabled && mode != ClientMode.TunFull;
    }

    private void UpdateTrayServerModeMenu(ServerMode? mode, string label, bool pending)
    {
        if (_serverModeMenu is not null)
        {
            _serverModeMenu.Text = $"Mode: {label}";
        }
        if (_serverModeProxyItem is null || _serverModeTunItem is null)
        {
            return;
        }
        var enabled = !pending;
        _serverModeProxyItem.Enabled = enabled && mode != ServerMode.Proxy;
        _serverModeTunItem.Enabled = enabled && mode != ServerMode.Tun;
    }

    private void RequestClientMode(ClientMode mode)
    {
        if (_modeManager is null)
        {
            return;
        }
        var result = _modeManager.ApplyClientMode(mode);
        SetStatus(result.Message);
        if (result.Success)
        {
            _pendingClientMode = mode;
            RefreshServiceStatus();
        }
    }

    private void RequestServerMode(ServerMode mode)
    {
        if (_modeManager is null)
        {
            return;
        }
        var result = _modeManager.ApplyServerMode(mode);
        SetStatus(result.Message);
        if (result.Success)
        {
            _pendingServerMode = mode;
            RefreshServiceStatus();
        }
    }
}
