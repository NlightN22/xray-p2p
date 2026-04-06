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

    public static string BuildTrayTooltip(ServiceStatusSnapshot snapshot, ClientRuntimeView? runtime)
    {
        var runtimeText = runtime?.Summary;
        var text = string.IsNullOrWhiteSpace(runtimeText)
            ? $"Client: {snapshot.ClientStatus}{Environment.NewLine}Server: {snapshot.ServerStatus}"
            : $"Client: {snapshot.ClientStatus} | {runtimeText}{Environment.NewLine}Server: {snapshot.ServerStatus}";
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

    public static ClientRuntimeView BuildClientRuntimeView(string serviceStatus, ClientStateFile? state)
    {
        var now = DateTimeOffset.UtcNow;
        if (state?.Runtime?.Timestamp is not DateTimeOffset timestamp)
        {
            return new ClientRuntimeView(ClientRuntimeStatus.Failed, "Tun: Unknown", "Tun: Unknown", null, false);
        }

        if (IsServiceStopped(serviceStatus))
        {
            return new ClientRuntimeView(ClientRuntimeStatus.Failed, "Tun: Stopped", "Tun: Stopped", state.Runtime.LastError, true);
        }
        if (IsServicePending(serviceStatus))
        {
            return new ClientRuntimeView(ClientRuntimeStatus.Pending, "Tun: Pending", "Tun: Pending", state.Runtime.LastError, true);
        }
        if (!IsServiceRunning(serviceStatus))
        {
            return new ClientRuntimeView(ClientRuntimeStatus.Failed, $"Tun: {serviceStatus}", $"Tun: {serviceStatus}", state.Runtime.LastError, true);
        }

        if (!state.TunEnabled)
        {
            return state.Runtime.SocksReady
                ? new ClientRuntimeView(ClientRuntimeStatus.Ready, "Proxy: Ready (SOCKS)", "Proxy: Ready (SOCKS)", null, true)
                : new ClientRuntimeView(ClientRuntimeStatus.Pending, "Proxy: Pending (SOCKS)", "Proxy: Pending (SOCKS)", state.Runtime.LastError, true);
        }

        var tun = state.Runtime.Tun;
        var routes = state.Runtime.Routes;
        var tunReady = tun?.Ready == true;
        var fullApplied = routes?.FullApplied == true;
        var redirectApplied = routes?.RedirectApplied == true;
        var routeLabel = fullApplied ? "Full" : redirectApplied ? "Split" : "Tun";
        var routeReady = fullApplied || redirectApplied;
        var summary = tunReady && routeReady ? $"Tun: Ready | Routes: {routeLabel}" : $"Tun: Pending | Routes: {routeLabel}";

        var detail = BuildTunDetail(tun, routes, routeLabel);
        var status = tunReady && routeReady ? ClientRuntimeStatus.Ready : ClientRuntimeStatus.Pending;
        return new ClientRuntimeView(status, summary, detail, state.Runtime.LastError, true);
    }

    private static string BuildTunDetail(RuntimeTunState? tun, RuntimeRoutesState? routes, string routeLabel)
    {
        var name = string.IsNullOrWhiteSpace(tun?.Name) ? "-" : tun.Name;
        var ip = string.IsNullOrWhiteSpace(tun?.IPv4) ? "-" : tun.IPv4;
        var prefix = tun?.Prefix > 0 ? $"/{tun.Prefix}" : "";
        var oper = string.IsNullOrWhiteSpace(tun?.OperStatus) ? "-" : tun.OperStatus;
        var dad = string.IsNullOrWhiteSpace(tun?.DadState) ? "-" : tun.DadState;
        var routesText = routeLabel switch
        {
            "Full" => $"full({routes?.FullBypassCount ?? 0})",
            "Split" => $"split({routes?.RedirectCount ?? 0})",
            _ => "none"
        };
        return $"Tun: {name} {ip}{prefix} {oper}/{dad} | Routes: {routesText}";
    }
}
