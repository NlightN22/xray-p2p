using System;
using ModernWpf;

namespace Xp2pUi;

internal readonly record struct ServiceButtonState(bool StartEnabled, bool StopEnabled);

internal static class UiLogic
{
    public static ApplicationTheme? GetThemeFromSelection(int selection)
    {
        return selection switch
        {
            1 => ApplicationTheme.Light,
            2 => ApplicationTheme.Dark,
            _ => null
        };
    }

    public static ServiceButtonState GetServiceButtonState(string status)
    {
        if (IsServiceRunning(status))
        {
            return new ServiceButtonState(false, true);
        }
        if (IsServiceStopped(status))
        {
            return new ServiceButtonState(true, false);
        }
        if (IsServicePending(status))
        {
            return new ServiceButtonState(false, false);
        }
        return new ServiceButtonState(true, true);
    }

    public static bool ShouldPromptStopServices(ServiceStatusSnapshot snapshot)
    {
        return IsServiceRunning(snapshot.ClientStatus) ||
            IsServiceRunning(snapshot.ServerStatus) ||
            IsServicePending(snapshot.ClientStatus) ||
            IsServicePending(snapshot.ServerStatus);
    }

    public static string BuildTrayTooltip(ServiceStatusSnapshot snapshot)
    {
        var text = $"Client: {snapshot.ClientStatus} | Server: {snapshot.ServerStatus}";
        return text.Length <= 63 ? text : text.Substring(0, 63);
    }

    public static bool IsServiceRunning(string status)
    {
        return string.Equals(status, "Running", StringComparison.OrdinalIgnoreCase);
    }

    public static bool IsServiceStopped(string status)
    {
        return string.Equals(status, "Stopped", StringComparison.OrdinalIgnoreCase);
    }

    public static bool IsServicePending(string status)
    {
        return string.Equals(status, "StartPending", StringComparison.OrdinalIgnoreCase) ||
            string.Equals(status, "StopPending", StringComparison.OrdinalIgnoreCase) ||
            string.Equals(status, "PausePending", StringComparison.OrdinalIgnoreCase) ||
            string.Equals(status, "ContinuePending", StringComparison.OrdinalIgnoreCase);
    }

    public static bool IsRestartEnabled(string status)
    {
        return !IsServicePending(status);
    }
}
