using Xunit;

namespace Xp2pUi.Tests;

public sealed class ModeLogicTests
{
    [Fact]
    public void ResolveClientMode_PrefersProxyWhenTunDisabled()
    {
        var state = new ClientStateFile
        {
            TunEnabled = false,
            Mode = "tun"
        };

        Assert.Equal(ClientMode.Proxy, ModeLogic.ResolveClientMode(state));
    }

    [Fact]
    public void ResolveClientMode_UsesExplicitFullMode()
    {
        var state = new ClientStateFile
        {
            TunEnabled = true,
            Mode = "full"
        };

        Assert.Equal(ClientMode.TunFull, ModeLogic.ResolveClientMode(state));
    }

    [Fact]
    public void ResolveClientMode_UsesRuntimeFlags()
    {
        var state = new ClientStateFile
        {
            TunEnabled = true,
            Runtime = new ClientRuntimeState
            {
                Routes = new RuntimeRoutesState
                {
                    FullApplied = true
                }
            }
        };

        Assert.Equal(ClientMode.TunFull, ModeLogic.ResolveClientMode(state));
    }

    [Fact]
    public void ResolveServerMode_FallsBackToTunEnabled()
    {
        var state = new ServerStateFile
        {
            TunEnabled = true
        };

        Assert.Equal(ServerMode.Tun, ModeLogic.ResolveServerMode(state));
    }

    [Fact]
    public void FormatClientMode_ReturnsLabels()
    {
        Assert.Equal("Proxy", ModeLogic.FormatClientMode(ClientMode.Proxy));
        Assert.Equal("Tun Split", ModeLogic.FormatClientMode(ClientMode.TunSplit));
        Assert.Equal("Tun Full", ModeLogic.FormatClientMode(ClientMode.TunFull));
    }
}
