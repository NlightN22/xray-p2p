using System;
using System.IO;
using Xunit;

namespace Xp2pUi.Tests;

public sealed class ModeManagerTests
{
    [Fact]
    public void ApplyClientMode_FullUsesSingleEndpointTag()
    {
        using var env = new TempConfigRoot();
        var manager = new ModeManager();

        File.WriteAllText(Path.Combine(env.Root, "xp2p-client.toml"),
            "[[client.endpoints]]\n" +
            "tag = \"proxy-alpha\"\n" +
            "hostname = \"edge.example\"\n");

        var result = manager.ApplyClientMode(ClientMode.TunFull);

        Assert.True(result.Success);
        var pendingPath = Path.Combine(env.Root, ".apply", "pending", "xp2p-client.toml");
        Assert.True(File.Exists(pendingPath));
        var content = File.ReadAllText(pendingPath);
        Assert.Contains("[client]", content, StringComparison.Ordinal);
        Assert.Contains("tun_enabled = true", content, StringComparison.Ordinal);
        Assert.Contains("tun_mode = \"full\"", content, StringComparison.Ordinal);
        Assert.Contains("full_tunnel_tag = \"proxy-alpha\"", content, StringComparison.Ordinal);
    }

    [Fact]
    public void ApplyClientMode_FullUsesInlineEndpointTag()
    {
        using var env = new TempConfigRoot();
        var manager = new ModeManager();

        File.WriteAllText(Path.Combine(env.Root, "xp2p-client.toml"),
            "[client]\n" +
            "endpoints = [{ tag = \"proxy-inline\", hostname = \"edge.example\" }]\n");

        var result = manager.ApplyClientMode(ClientMode.TunFull);

        Assert.True(result.Success);
        var pendingPath = Path.Combine(env.Root, ".apply", "pending", "xp2p-client.toml");
        Assert.True(File.Exists(pendingPath));
        var content = File.ReadAllText(pendingPath);
        Assert.Contains("full_tunnel_tag = \"proxy-inline\"", content, StringComparison.Ordinal);
    }

    [Fact]
    public void ApplyClientMode_FullFailsWhenMultipleEndpoints()
    {
        using var env = new TempConfigRoot();
        var manager = new ModeManager();

        File.WriteAllText(Path.Combine(env.Root, "xp2p-client.toml"),
            "[[client.endpoints]]\n" +
            "tag = \"proxy-alpha\"\n" +
            "hostname = \"edge.example\"\n" +
            "\n" +
            "[[client.endpoints]]\n" +
            "tag = \"proxy-beta\"\n" +
            "hostname = \"edge2.example\"\n");

        var result = manager.ApplyClientMode(ClientMode.TunFull);

        Assert.False(result.Success);
        var pendingPath = Path.Combine(env.Root, ".apply", "pending", "xp2p-client.toml");
        Assert.False(File.Exists(pendingPath));
    }

    [Fact]
    public void ApplyClientMode_FullWithOverrideTagWritesPending()
    {
        using var env = new TempConfigRoot();
        var manager = new ModeManager();

        File.WriteAllText(Path.Combine(env.Root, "xp2p-client.toml"),
            "[[client.endpoints]]\n" +
            "tag = \"proxy-alpha\"\n" +
            "hostname = \"edge.example\"\n" +
            "\n" +
            "[[client.endpoints]]\n" +
            "tag = \"proxy-beta\"\n" +
            "hostname = \"edge2.example\"\n");

        var result = manager.ApplyClientMode(ClientMode.TunFull, "proxy-beta");

        Assert.True(result.Success);
        var pendingPath = Path.Combine(env.Root, ".apply", "pending", "xp2p-client.toml");
        Assert.True(File.Exists(pendingPath));
        var content = File.ReadAllText(pendingPath);
        Assert.Contains("full_tunnel_tag = \"proxy-beta\"", content, StringComparison.Ordinal);
    }

    [Fact]
    public void ApplyClientMode_SplitWritesPending()
    {
        using var env = new TempConfigRoot();
        var manager = new ModeManager();

        var result = manager.ApplyClientMode(ClientMode.TunSplit);

        Assert.True(result.Success);
        var pendingPath = Path.Combine(env.Root, ".apply", "pending", "xp2p-client.toml");
        Assert.True(File.Exists(pendingPath));
        var content = File.ReadAllText(pendingPath);
        Assert.Contains("[client]", content, StringComparison.Ordinal);
        Assert.Contains("tun_enabled = true", content, StringComparison.Ordinal);
        Assert.Contains("tun_mode = \"split\"", content, StringComparison.Ordinal);
        var requestPath = Path.Combine(env.Root, ".apply", "apply.request");
        Assert.True(File.Exists(requestPath));
    }

    [Fact]
    public void ApplyClientMode_ProxyWritesPending()
    {
        using var env = new TempConfigRoot();
        var manager = new ModeManager();

        var result = manager.ApplyClientMode(ClientMode.Proxy);

        Assert.True(result.Success);
        var pendingPath = Path.Combine(env.Root, ".apply", "pending", "xp2p-client.toml");
        Assert.True(File.Exists(pendingPath));
        var content = File.ReadAllText(pendingPath);
        Assert.Contains("[client]", content, StringComparison.Ordinal);
        Assert.Contains("tun_enabled = false", content, StringComparison.Ordinal);
    }

    [Fact]
    public void ApplyServerMode_TunWritesPending()
    {
        using var env = new TempConfigRoot();
        var manager = new ModeManager();

        var result = manager.ApplyServerMode(ServerMode.Tun);

        Assert.True(result.Success);
        var pendingPath = Path.Combine(env.Root, ".apply", "pending", "xp2p-server.toml");
        Assert.True(File.Exists(pendingPath));
        var content = File.ReadAllText(pendingPath);
        Assert.Contains("[server]", content, StringComparison.Ordinal);
        Assert.Contains("tun_enabled = true", content, StringComparison.Ordinal);
        var requestPath = Path.Combine(env.Root, ".apply", "apply.request");
        Assert.True(File.Exists(requestPath));
    }

    private sealed class TempConfigRoot : IDisposable
    {
        private readonly string? _previousRoot;
        private readonly string? _previousLogRoot;

        public TempConfigRoot()
        {
            _previousRoot = Environment.GetEnvironmentVariable("XP2P_CONFIG_ROOT");
            _previousLogRoot = Environment.GetEnvironmentVariable("XP2P_LOG_ROOT");
            Root = Path.Combine(Path.GetTempPath(), $"xp2p-ui-tests-{Guid.NewGuid():N}");
            Directory.CreateDirectory(Root);
            Environment.SetEnvironmentVariable("XP2P_CONFIG_ROOT", Root);
            Environment.SetEnvironmentVariable("XP2P_LOG_ROOT", Path.Combine(Root, "logs"));
        }

        public string Root { get; }

        public void Dispose()
        {
            Environment.SetEnvironmentVariable("XP2P_CONFIG_ROOT", _previousRoot);
            Environment.SetEnvironmentVariable("XP2P_LOG_ROOT", _previousLogRoot);
            try
            {
                if (Directory.Exists(Root))
                {
                    Directory.Delete(Root, true);
                }
            }
            catch
            {
                // Ignore cleanup failures.
            }
        }
    }
}
