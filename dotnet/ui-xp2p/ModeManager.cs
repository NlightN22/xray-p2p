using System;
using System.IO;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using System.Text.RegularExpressions;

namespace Xp2pUi;

internal sealed class ModeManager
{
    private readonly Action<string>? _log;

    private const string ConfigRootEnv = "XP2P_CONFIG_ROOT";
    private const string LogRootEnv = "XP2P_LOG_ROOT";

    public ModeManager(Action<string>? log = null)
    {
        _log = log;
    }

    public OperationResult ApplyClientMode(ClientMode mode, string? fullTunnelTagOverride = null)
    {
        var desiredTunEnabled = mode != ClientMode.Proxy;
        var desiredTunMode = mode == ClientMode.TunFull ? "full" : "split";
        var configPath = GetConfigPath("xp2p-client.toml");
        var legacyPendingPath = GetPendingConfigPath("xp2p-client.toml");
        var sourcePath = ResolveSourceConfig(configPath, legacyPendingPath);

        Log($"mode manager: client mode request {ModeLogic.FormatClientMode(mode)}");
        Log($"mode manager: client config source={sourcePath} desired={configPath}");

        var content = ReadTextOrEmpty(sourcePath);
        content = UpdateTomlValue(content, "client", "tun_enabled", desiredTunEnabled ? "true" : "false");
        if (desiredTunEnabled)
        {
            if (mode == ClientMode.TunFull)
            {
                var existingTag = ReadTomlValue(content, "client", "full_tunnel_tag");
                var resolvedTag = string.IsNullOrWhiteSpace(fullTunnelTagOverride) ? existingTag : fullTunnelTagOverride;
                if (string.IsNullOrWhiteSpace(resolvedTag))
                {
                    var tags = ReadEndpointTags(content);
                    Log($"mode manager: client endpoint tags count={tags.Count} tags={string.Join(",", tags)}");
                    if (tags.Count == 0)
                    {
                        Log("mode manager: client full mode rejected (no endpoints found)");
                        return OperationResult.Fail("Full mode requires endpoint tag; no endpoints found.");
                    }
                    if (tags.Count > 1)
                    {
                        Log("mode manager: client full mode rejected (multiple endpoints found)");
                        return OperationResult.Fail("Full mode requires endpoint tag; multiple endpoints found.");
                    }
                    resolvedTag = tags[0];
                    Log($"mode manager: client full mode auto tag={resolvedTag}");
                }
                if (!string.IsNullOrWhiteSpace(resolvedTag))
                {
                    content = UpdateTomlValue(content, "client", "full_tunnel_tag", $"\"{resolvedTag}\"");
                }
            }
            content = UpdateTomlValue(content, "client", "tun_mode", $"\"{desiredTunMode}\"");
        }

        try
        {
            WriteFileWithAudit(configPath, content, ignoreAuditErrors: false);
            Log($"mode manager: client desired config written {configPath}");
            WriteApplyRequest("client");
            return OperationResult.Ok($"Client mode requested: {ModeLogic.FormatClientMode(mode)}.");
        }
        catch (Exception ex)
        {
            Log($"mode manager: client mode update failed {ex.GetType().Name} {ex.Message}");
            return OperationResult.Fail($"Client mode update failed: {ex.Message}");
        }
    }

    public OperationResult ApplyServerMode(ServerMode mode)
    {
        var desiredTunEnabled = mode == ServerMode.Tun;
        var configPath = GetConfigPath("xp2p-server.toml");
        var legacyPendingPath = GetPendingConfigPath("xp2p-server.toml");
        var sourcePath = ResolveSourceConfig(configPath, legacyPendingPath);

        Log($"mode manager: server mode request {ModeLogic.FormatServerMode(mode)}");
        Log($"mode manager: server config source={sourcePath} desired={configPath}");

        var content = ReadTextOrEmpty(sourcePath);
        content = UpdateTomlValue(content, "server", "tun_enabled", desiredTunEnabled ? "true" : "false");

        try
        {
            WriteFileWithAudit(configPath, content, ignoreAuditErrors: false);
            Log($"mode manager: server desired config written {configPath}");
            WriteApplyRequest("server");
            return OperationResult.Ok($"Server mode requested: {ModeLogic.FormatServerMode(mode)}.");
        }
        catch (Exception ex)
        {
            Log($"mode manager: server mode update failed {ex.GetType().Name} {ex.Message}");
            return OperationResult.Fail($"Server mode update failed: {ex.Message}");
        }
    }

    public string GetClientStatePath()
    {
        return Path.Combine(GetConfigRoot(), "xp2p-client.state.json");
    }

    public string GetServerStatePath()
    {
        return Path.Combine(GetConfigRoot(), "xp2p-server.state.json");
    }

    private static string ResolveSourceConfig(string configPath, string pendingPath)
    {
        if (File.Exists(configPath))
        {
            return configPath;
        }
        if (File.Exists(pendingPath))
        {
            return pendingPath;
        }
        return configPath;
    }

    private static string ReadTextOrEmpty(string path)
    {
        if (string.IsNullOrWhiteSpace(path) || !File.Exists(path))
        {
            return "";
        }
        return File.ReadAllText(path);
    }

    private static string UpdateTomlValue(string text, string section, string key, string value)
    {
        var normalized = NormalizeLineEndings(text);
        var lines = normalized.Length == 0 ? new string[0] : normalized.Split('\n');
        var sectionStart = -1;
        var sectionEnd = lines.Length;

        for (var i = 0; i < lines.Length; i++)
        {
            var trimmed = lines[i].Trim();
            if (!trimmed.StartsWith("[") || !trimmed.EndsWith("]"))
            {
                continue;
            }
            var name = trimmed.Substring(1, trimmed.Length - 2).Trim();
            if (string.Equals(name, section, StringComparison.OrdinalIgnoreCase))
            {
                sectionStart = i;
                continue;
            }
            if (sectionStart != -1)
            {
                sectionEnd = i;
                break;
            }
        }

        if (sectionStart == -1)
        {
            var builder = new StringBuilder(normalized.Length + 64);
            if (!string.IsNullOrWhiteSpace(normalized))
            {
                builder.Append(normalized.TrimEnd('\n'));
                builder.Append('\n');
                builder.Append('\n');
            }
            builder.Append('[').Append(section).Append(']').Append('\n');
            builder.Append(key).Append(" = ").Append(value).Append('\n');
            return builder.ToString();
        }

        for (var i = sectionStart + 1; i < sectionEnd; i++)
        {
            if (MatchesKey(lines[i], key))
            {
                lines[i] = $"{key} = {value}";
                return EnsureLineEnding(lines);
            }
        }

        var insert = sectionEnd;
        var extended = new string[lines.Length + 1];
        Array.Copy(lines, 0, extended, 0, insert);
        extended[insert] = $"{key} = {value}";
        Array.Copy(lines, insert, extended, insert + 1, lines.Length - insert);
        return EnsureLineEnding(extended);
    }

    private static bool MatchesKey(string line, string key)
    {
        if (string.IsNullOrWhiteSpace(line))
        {
            return false;
        }
        var trimmed = line.TrimStart();
        if (trimmed.StartsWith("#"))
        {
            return false;
        }
        if (!trimmed.StartsWith(key, StringComparison.Ordinal))
        {
            return false;
        }
        var rest = trimmed.Substring(key.Length);
        if (rest.Length == 0)
        {
            return false;
        }
        var next = rest[0];
        return char.IsWhiteSpace(next) || next == '=';
    }

    private static string? ReadTomlValue(string text, string section, string key)
    {
        var normalized = NormalizeLineEndings(text);
        var lines = normalized.Length == 0 ? Array.Empty<string>() : normalized.Split('\n');
        var inSection = false;

        foreach (var line in lines)
        {
            var trimmed = line.Trim();
            if (trimmed.StartsWith("[") && trimmed.EndsWith("]"))
            {
                var name = trimmed.Substring(1, trimmed.Length - 2).Trim();
                inSection = string.Equals(name, section, StringComparison.OrdinalIgnoreCase);
                continue;
            }
            if (!inSection)
            {
                continue;
            }
            if (!MatchesKey(line, key))
            {
                continue;
            }
            var idx = line.IndexOf('=');
            if (idx < 0)
            {
                return null;
            }
            var value = line[(idx + 1)..].Trim();
            var commentIdx = value.IndexOf('#');
            if (commentIdx >= 0)
            {
                value = value[..commentIdx].Trim();
            }
            return TrimTomlQuotes(value);
        }
        return null;
    }

    public ClientFullTunnelTagState GetClientFullTunnelTagState()
    {
        var configPath = GetConfigPath("xp2p-client.toml");
        var legacyPendingPath = GetPendingConfigPath("xp2p-client.toml");
        var sourcePath = ResolveSourceConfig(configPath, legacyPendingPath);
        var content = ReadTextOrEmpty(sourcePath);
        var existing = ReadTomlValue(content, "client", "full_tunnel_tag") ?? "";
        var tags = ReadEndpointTags(content);
        return new ClientFullTunnelTagState(existing, tags);
    }

    private static System.Collections.Generic.List<string> ReadEndpointTags(string content)
    {
        var normalized = NormalizeLineEndings(content);
        var lines = normalized.Length == 0 ? Array.Empty<string>() : normalized.Split('\n');
        var inEndpoints = false;
        var tags = new System.Collections.Generic.List<string>();
        var seen = new System.Collections.Generic.HashSet<string>(StringComparer.OrdinalIgnoreCase);

        var endpointsBlock = ExtractEndpointsInlineBlock(lines);
        if (!string.IsNullOrWhiteSpace(endpointsBlock))
        {
            foreach (Match match in Regex.Matches(endpointsBlock, @"\btag\s*=\s*(['""])(?<tag>[^'""]+)\1"))
            {
                var value = match.Groups["tag"].Value.Trim();
                if (value.Length == 0)
                {
                    continue;
                }
                if (seen.Add(value))
                {
                    tags.Add(value);
                }
            }
        }

        foreach (var line in lines)
        {
            var trimmed = line.Trim();
            if (trimmed.StartsWith("[") && trimmed.EndsWith("]"))
            {
                var header = trimmed.Trim('[', ']').Trim();
                if (string.Equals(header, "client.endpoints", StringComparison.OrdinalIgnoreCase))
                {
                    inEndpoints = true;
                    continue;
                }
                if (inEndpoints)
                {
                    inEndpoints = false;
                }
                continue;
            }
            if (!inEndpoints)
            {
                continue;
            }
            if (!MatchesKey(line, "tag"))
            {
                continue;
            }
            var value = ReadValueFromLine(line);
            if (string.IsNullOrWhiteSpace(value))
            {
                continue;
            }
            if (seen.Add(value))
            {
                tags.Add(value);
            }
        }
        return tags;
    }

    private static string ExtractEndpointsInlineBlock(string[] lines)
    {
        var capture = false;
        var depth = 0;
        var builder = new StringBuilder();
        foreach (var line in lines)
        {
            var trimmed = line.Trim();
            if (!capture && Regex.IsMatch(trimmed, @"^endpoints\s*="))
            {
                capture = true;
            }
            if (!capture)
            {
                continue;
            }
            builder.Append(trimmed).Append(' ');
            foreach (var ch in trimmed)
            {
                if (ch == '[')
                {
                    depth++;
                }
                else if (ch == ']')
                {
                    depth--;
                }
            }
            if (capture && depth <= 0 && trimmed.Contains(']'))
            {
                break;
            }
        }
        return builder.ToString();
    }

    private static string ReadValueFromLine(string line)
    {
        var idx = line.IndexOf('=');
        if (idx < 0)
        {
            return "";
        }
        var value = line[(idx + 1)..].Trim();
        var commentIdx = value.IndexOf('#');
        if (commentIdx >= 0)
        {
            value = value[..commentIdx].Trim();
        }
        return TrimTomlQuotes(value);
    }

    private void WriteApplyRequest(string role)
    {
        var path = GetApplyRequestPath();
        var desiredRole = NormalizeRole(role);
        if (TryReadApplyRequest(path, out var existing, out var exists))
        {
            if (exists && MatchesRole(existing.Role, desiredRole))
            {
                Log($"mode manager: apply request already matches role={desiredRole} path={path}");
                return;
            }
            if (exists && RequiresAnyRole(existing.Role, desiredRole))
            {
                desiredRole = "any";
            }
        }

        var request = new ApplyRequest
        {
            Id = Guid.NewGuid().ToString(),
            Timestamp = DateTimeOffset.UtcNow,
            Role = desiredRole
        };
        var json = JsonSerializer.Serialize(request, new JsonSerializerOptions { WriteIndented = true });
        json = json.TrimEnd() + "\n";
        WriteFileWithAudit(path, json, ignoreAuditErrors: true);
        Log($"mode manager: apply request written role={desiredRole} path={path}");
    }

    private static bool TryReadApplyRequest(string path, out ApplyRequest existing, out bool exists)
    {
        existing = new ApplyRequest();
        exists = false;
        if (string.IsNullOrWhiteSpace(path) || !File.Exists(path))
        {
            return true;
        }
        try
        {
            var text = File.ReadAllText(path);
            exists = true;
            if (string.IsNullOrWhiteSpace(text))
            {
                return true;
            }
            existing = JsonSerializer.Deserialize<ApplyRequest>(text) ?? new ApplyRequest();
            existing.Role = NormalizeRole(existing.Role);
            return true;
        }
        catch
        {
            return false;
        }
    }

    private static bool MatchesRole(string existingRole, string desiredRole)
    {
        var existing = NormalizeRole(existingRole);
        var desired = NormalizeRole(desiredRole);
        if (string.IsNullOrWhiteSpace(desired))
        {
            return false;
        }
        if (string.IsNullOrWhiteSpace(existing))
        {
            return true;
        }
        if (string.Equals(existing, desired, StringComparison.OrdinalIgnoreCase))
        {
            return true;
        }
        return string.Equals(existing, "any", StringComparison.OrdinalIgnoreCase);
    }

    private static bool RequiresAnyRole(string existingRole, string desiredRole)
    {
        var existing = NormalizeRole(existingRole);
        var desired = NormalizeRole(desiredRole);
        if (string.IsNullOrWhiteSpace(existing) || string.IsNullOrWhiteSpace(desired))
        {
            return false;
        }
        if (string.Equals(existing, "any", StringComparison.OrdinalIgnoreCase) ||
            string.Equals(desired, "any", StringComparison.OrdinalIgnoreCase))
        {
            return false;
        }
        return !string.Equals(existing, desired, StringComparison.OrdinalIgnoreCase);
    }

    private static string NormalizeRole(string? role)
    {
        return string.IsNullOrWhiteSpace(role) ? "" : role.Trim().ToLowerInvariant();
    }

    private void WriteFileWithAudit(string path, string content, bool ignoreAuditErrors)
    {
        var data = Encoding.UTF8.GetBytes(NormalizeLineEndings(content));
        var (oldHash, oldSize) = ReadFileHash(path);
        var newHash = HashBytes(data);
        if (oldHash == newHash)
        {
            Log($"mode manager: skip write (no changes) path={path}");
            return;
        }
        var dir = Path.GetDirectoryName(path);
        if (!string.IsNullOrWhiteSpace(dir))
        {
            Directory.CreateDirectory(dir);
        }
        WriteAtomic(path, data);
        var auditPath = GetAuditLogPath();
        if (string.IsNullOrWhiteSpace(auditPath))
        {
            return;
        }
        try
        {
            AppendAudit(auditPath, path, oldHash, newHash, oldSize, data.LongLength);
        }
        catch when (ignoreAuditErrors)
        {
            Log($"mode manager: audit write skipped path={auditPath}");
        }
    }

    private static void WriteAtomic(string path, byte[] data)
    {
        var dir = Path.GetDirectoryName(path) ?? ".";
        var tempName = $".tmp-{Environment.ProcessId}-{DateTimeOffset.UtcNow.ToUnixTimeMilliseconds()}";
        var tempPath = Path.Combine(dir, tempName);
        File.WriteAllBytes(tempPath, data);
        File.Move(tempPath, path, true);
    }

    private void AppendAudit(string auditPath, string path, string oldHash, string newHash, long oldSize, long newSize)
    {
        var dir = Path.GetDirectoryName(auditPath);
        if (!string.IsNullOrWhiteSpace(dir))
        {
            Directory.CreateDirectory(dir);
        }
        var line = $"{DateTimeOffset.UtcNow:O} user={Environment.UserName} file={path} old_hash={oldHash} new_hash={newHash} old_size={oldSize} new_size={newSize} cmd={Environment.CommandLine}\n";
        File.AppendAllText(auditPath, line, Encoding.UTF8);
    }

    private static (string Hash, long Size) ReadFileHash(string path)
    {
        if (!File.Exists(path))
        {
            return ("", 0);
        }
        var data = File.ReadAllBytes(path);
        return (HashBytes(data), data.LongLength);
    }

    private static string HashBytes(byte[] data)
    {
        var hash = SHA256.HashData(data);
        var builder = new StringBuilder(hash.Length * 2);
        foreach (var b in hash)
        {
            _ = builder.Append(b.ToString("x2"));
        }
        return builder.ToString();
    }

    private string GetConfigRoot()
    {
        var overrideValue = Environment.GetEnvironmentVariable(ConfigRootEnv);
        if (!string.IsNullOrWhiteSpace(overrideValue))
        {
            return overrideValue.Trim();
        }
        var programData = Environment.GetFolderPath(Environment.SpecialFolder.CommonApplicationData);
        if (string.IsNullOrWhiteSpace(programData))
        {
            programData = @"C:\ProgramData";
        }
        return Path.Combine(programData, "xp2p");
    }

    private string GetLogRoot()
    {
        var overrideValue = Environment.GetEnvironmentVariable(LogRootEnv);
        if (!string.IsNullOrWhiteSpace(overrideValue))
        {
            return overrideValue.Trim();
        }
        return Path.Combine(GetConfigRoot(), "logs");
    }

    private string GetAuditLogPath()
    {
        return Path.Combine(GetLogRoot(), "audit.log");
    }

    private string GetConfigPath(string fileName)
    {
        return Path.Combine(GetConfigRoot(), fileName);
    }

    private string GetPendingConfigPath(string fileName)
    {
        return Path.Combine(GetConfigRoot(), ".apply", "pending", fileName);
    }

    private string GetStateRoot()
    {
        return Path.Combine(GetConfigRoot(), ".state");
    }

    private string GetApplyRequestPath()
    {
        return Path.Combine(GetStateRoot(), "apply.request");
    }

    private static string NormalizeLineEndings(string text)
    {
        if (string.IsNullOrEmpty(text))
        {
            return "";
        }
        var normalized = text.Replace("\r\n", "\n").Replace("\r", "\n");
        if (!normalized.EndsWith("\n", StringComparison.Ordinal))
        {
            normalized += "\n";
        }
        return normalized;
    }

    private static string EnsureLineEnding(string[] lines)
    {
        if (lines.Length == 0)
        {
            return "\n";
        }
        var content = string.Join("\n", lines);
        return NormalizeLineEndings(content);
    }

    private static string TrimTomlQuotes(string value)
    {
        if (value.Length < 2)
        {
            return value;
        }
        if ((value.StartsWith("\"") && value.EndsWith("\"")) ||
            (value.StartsWith("'") && value.EndsWith("'")))
        {
            return value.Substring(1, value.Length - 2);
        }
        return value;
    }

    private void Log(string message)
    {
        _log?.Invoke(message);
    }

    public readonly record struct ClientFullTunnelTagState(
        string ExistingTag,
        System.Collections.Generic.IReadOnlyList<string> CandidateTags);

    private sealed class ApplyRequest
    {
        public string Id { get; set; } = "";
        public DateTimeOffset Timestamp { get; set; }
        public string Role { get; set; } = "";
    }
}
