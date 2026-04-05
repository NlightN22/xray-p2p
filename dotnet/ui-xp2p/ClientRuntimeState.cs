using System;
using System.IO;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace Xp2pUi;

internal enum ClientRuntimeStatus
{
    Ready,
    Pending,
    Failed
}

internal sealed record ClientStateFile
{
    [JsonPropertyName("tun_enabled")]
    public bool TunEnabled { get; init; }

    [JsonPropertyName("mode")]
    public string? Mode { get; init; }

    [JsonPropertyName("runtime")]
    public ClientRuntimeState? Runtime { get; init; }
}

internal sealed record ClientRuntimeState
{
    [JsonPropertyName("tun")]
    public RuntimeTunState? Tun { get; init; }

    [JsonPropertyName("routes")]
    public RuntimeRoutesState? Routes { get; init; }

    [JsonPropertyName("socks_ready")]
    public bool SocksReady { get; init; }

    [JsonPropertyName("last_error")]
    public string? LastError { get; init; }

    [JsonPropertyName("timestamp")]
    public DateTimeOffset? Timestamp { get; init; }
}

internal sealed record RuntimeTunState
{
    [JsonPropertyName("name")]
    public string? Name { get; init; }

    [JsonPropertyName("if_index")]
    public int IfIndex { get; init; }

    [JsonPropertyName("ipv4")]
    public string? IPv4 { get; init; }

    [JsonPropertyName("prefix")]
    public int Prefix { get; init; }

    [JsonPropertyName("oper_status")]
    public string? OperStatus { get; init; }

    [JsonPropertyName("dad_state")]
    public string? DadState { get; init; }

    [JsonPropertyName("ready")]
    public bool Ready { get; init; }

    [JsonPropertyName("last_error")]
    public string? LastError { get; init; }
}

internal sealed record RuntimeRoutesState
{
    [JsonPropertyName("redirect_applied")]
    public bool RedirectApplied { get; init; }

    [JsonPropertyName("redirect_count")]
    public int RedirectCount { get; init; }

    [JsonPropertyName("full_applied")]
    public bool FullApplied { get; init; }

    [JsonPropertyName("full_bypass_count")]
    public int FullBypassCount { get; init; }
}

internal sealed record ClientRuntimeView(
    ClientRuntimeStatus Status,
    string Summary,
    string Detail,
    string? LastError,
    bool IsFresh);

internal static class ClientStateReader
{
    private static readonly JsonSerializerOptions JsonOptions = new()
    {
        PropertyNameCaseInsensitive = true
    };

    public static ClientStateFile? TryLoad(string path)
    {
        if (string.IsNullOrWhiteSpace(path))
        {
            return null;
        }
        if (!File.Exists(path))
        {
            return null;
        }
        try
        {
            var json = File.ReadAllText(path);
            if (string.IsNullOrWhiteSpace(json))
            {
                return null;
            }
            return JsonSerializer.Deserialize<ClientStateFile>(json, JsonOptions);
        }
        catch
        {
            return null;
        }
    }
}
