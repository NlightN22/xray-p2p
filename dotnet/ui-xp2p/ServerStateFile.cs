using System;
using System.IO;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace Xp2pUi;

internal sealed record ServerStateFile
{
    [JsonPropertyName("tun_enabled")]
    public bool TunEnabled { get; init; }

    [JsonPropertyName("mode")]
    public string? Mode { get; init; }

    [JsonPropertyName("timestamp")]
    public DateTimeOffset? Timestamp { get; init; }
}

internal static class ServerStateReader
{
    private static readonly JsonSerializerOptions JsonOptions = new()
    {
        PropertyNameCaseInsensitive = true
    };

    public static ServerStateFile? TryLoad(string path)
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
            return JsonSerializer.Deserialize<ServerStateFile>(json, JsonOptions);
        }
        catch
        {
            return null;
        }
    }
}
