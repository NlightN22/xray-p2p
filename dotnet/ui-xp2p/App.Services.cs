using System;
using System.Diagnostics;
using System.IO;
using System.Windows;

namespace Xp2pUi;

internal sealed partial class App
{
    private async void RequestShutdown()
    {
        if (ShouldPromptStopServices())
        {
            var result = Dispatcher.Invoke(() => MessageBox.Show(
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
}
