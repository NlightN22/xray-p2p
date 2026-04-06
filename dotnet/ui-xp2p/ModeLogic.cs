using System;

namespace Xp2pUi;

internal enum ClientMode
{
    Proxy,
    TunSplit,
    TunFull
}

internal enum ServerMode
{
    Proxy,
    Tun
}

internal static class ModeLogic
{
    public static ClientMode? ResolveClientMode(ClientStateFile? state)
    {
        if (state is null)
        {
            return null;
        }
        if (!state.TunEnabled)
        {
            return ClientMode.Proxy;
        }
        var mode = Normalize(state.Mode);
        if (mode == "proxy")
        {
            return ClientMode.Proxy;
        }
        if (mode is "full" or "tun-full")
        {
            return ClientMode.TunFull;
        }
        if (mode is "split" or "tun-split")
        {
            return ClientMode.TunSplit;
        }
        if (state.Runtime?.Routes?.FullApplied == true)
        {
            return ClientMode.TunFull;
        }
        if (state.Runtime?.Routes?.RedirectApplied == true)
        {
            return ClientMode.TunSplit;
        }
        return ClientMode.TunSplit;
    }

    public static ServerMode? ResolveServerMode(ServerStateFile? state)
    {
        if (state is null)
        {
            return null;
        }
        var mode = Normalize(state.Mode);
        if (mode == "proxy")
        {
            return ServerMode.Proxy;
        }
        if (mode == "tun")
        {
            return ServerMode.Tun;
        }
        return state.TunEnabled ? ServerMode.Tun : ServerMode.Proxy;
    }

    public static string FormatClientMode(ClientMode mode)
    {
        return mode switch
        {
            ClientMode.Proxy => "Proxy",
            ClientMode.TunSplit => "Tun Split",
            ClientMode.TunFull => "Tun Full",
            _ => "Unknown"
        };
    }

    public static string FormatServerMode(ServerMode mode)
    {
        return mode switch
        {
            ServerMode.Proxy => "Proxy",
            ServerMode.Tun => "Tun",
            _ => "Unknown"
        };
    }

    public static string FormatPending(string label)
    {
        return $"{label} (Pending)";
    }

    public static ClientMode[] ClientAlternatives(ClientMode? current)
    {
        if (current is null)
        {
            return new[] { ClientMode.Proxy, ClientMode.TunSplit, ClientMode.TunFull };
        }
        return current.Value switch
        {
            ClientMode.Proxy => new[] { ClientMode.TunSplit, ClientMode.TunFull },
            ClientMode.TunSplit => new[] { ClientMode.Proxy, ClientMode.TunFull },
            ClientMode.TunFull => new[] { ClientMode.Proxy, ClientMode.TunSplit },
            _ => new[] { ClientMode.Proxy, ClientMode.TunSplit, ClientMode.TunFull }
        };
    }

    public static ServerMode[] ServerAlternatives(ServerMode? current)
    {
        if (current is null)
        {
            return new[] { ServerMode.Proxy, ServerMode.Tun };
        }
        return current.Value switch
        {
            ServerMode.Proxy => new[] { ServerMode.Tun },
            ServerMode.Tun => new[] { ServerMode.Proxy },
            _ => new[] { ServerMode.Proxy, ServerMode.Tun }
        };
    }

    private static string Normalize(string? value)
    {
        return string.IsNullOrWhiteSpace(value) ? "" : value.Trim().ToLowerInvariant();
    }
}
