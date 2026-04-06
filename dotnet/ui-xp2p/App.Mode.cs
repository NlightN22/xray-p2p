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
        if (mode == ClientMode.TunFull)
        {
            var state = _modeManager.GetClientFullTunnelTagState();
            if (!string.IsNullOrWhiteSpace(state.ExistingTag))
            {
                var resultExisting = _modeManager.ApplyClientMode(mode);
                SetStatus(resultExisting.Message);
                if (resultExisting.Success)
                {
                    _pendingClientMode = mode;
                    RefreshServiceStatus();
                }
                return;
            }
            if (state.CandidateTags.Count == 1)
            {
                var resultSingle = _modeManager.ApplyClientMode(mode, state.CandidateTags[0]);
                SetStatus(resultSingle.Message);
                if (resultSingle.Success)
                {
                    _pendingClientMode = mode;
                    RefreshServiceStatus();
                }
                return;
            }
            if (state.CandidateTags.Count > 1 && _window is not null)
            {
                var dialog = new TagSelectionDialog(state.CandidateTags)
                {
                    Owner = _window
                };
                if (dialog.ShowDialog() == true && !string.IsNullOrWhiteSpace(dialog.SelectedTag))
                {
                    var resultPick = _modeManager.ApplyClientMode(mode, dialog.SelectedTag);
                    SetStatus(resultPick.Message);
                    if (resultPick.Success)
                    {
                        _pendingClientMode = mode;
                        RefreshServiceStatus();
                    }
                }
                else
                {
                    SetStatus("Full mode change cancelled.");
                }
                return;
            }
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
