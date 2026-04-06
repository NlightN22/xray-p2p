using System;
using System.IO;
using System.Windows.Threading;

namespace Xp2pUi;

internal sealed partial class App
{
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
        RefreshServerModeStatus();
        Log($"service status changed: client={snapshot.ClientStatus} server={snapshot.ServerStatus} busy={_serviceManager?.IsBusy}");
        UpdateTrayStatusLabels(snapshot);
        UpdateTrayIconFromStatus();
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
        RefreshServerModeStatus();
        UpdateTrayStatusLabels(snapshot);
        UpdateTrayIconFromStatus();
    }

    private void RefreshClientRuntimeStatus(ServiceStatusSnapshot snapshot)
    {
        if (_window is null)
        {
            return;
        }
        var statePath = _modeManager?.GetClientStatePath() ?? GetClientStatePath();
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
        var mode = ModeLogic.ResolveClientMode(state);
        if (_pendingClientMode.HasValue && mode.HasValue && mode.Value == _pendingClientMode.Value)
        {
            _pendingClientMode = null;
        }
        var pending = _pendingClientMode;
        var label = mode.HasValue ? ModeLogic.FormatClientMode(mode.Value) : "Unknown";
        var display = pending.HasValue ? ModeLogic.FormatPending(ModeLogic.FormatClientMode(pending.Value)) : label;
        Dispatcher.Invoke(() =>
        {
            _window.SetClientRuntimeStatus(message);
            _window.SetClientModeStatus(mode, display, pending.HasValue);
        });
        UpdateTrayClientModeMenu(mode, display, pending.HasValue);
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

    private void RefreshServerModeStatus()
    {
        if (_window is null)
        {
            return;
        }
        var statePath = _modeManager?.GetServerStatePath();
        if (string.IsNullOrWhiteSpace(statePath))
        {
            return;
        }
        var state = ServerStateReader.TryLoad(statePath);
        var mode = ModeLogic.ResolveServerMode(state);
        if (_pendingServerMode.HasValue && mode.HasValue && mode.Value == _pendingServerMode.Value)
        {
            _pendingServerMode = null;
        }
        var pending = _pendingServerMode;
        var label = mode.HasValue ? ModeLogic.FormatServerMode(mode.Value) : "Unknown";
        var display = pending.HasValue ? ModeLogic.FormatPending(ModeLogic.FormatServerMode(pending.Value)) : label;
        Dispatcher.Invoke(() => _window.SetServerModeStatus(mode, display, pending.HasValue));
        UpdateTrayServerModeMenu(mode, display, pending.HasValue);
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

    private void SetStatus(string message)
    {
        if (_window is null)
        {
            return;
        }
        Dispatcher.Invoke(() => _window.SetStatus(message));
    }

    private bool ShouldPromptStopServices()
    {
        if (_serviceManager is null)
        {
            return false;
        }
        return UiLogic.ShouldPromptStopServices(_serviceManager.GetSnapshot());
    }
}
