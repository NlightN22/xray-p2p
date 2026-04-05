using ModernWpf;
using Xunit;

namespace Xp2pUi.Tests;

public sealed class UiLogicTests
{
    [Theory]
    [InlineData(1, ApplicationTheme.Light)]
    [InlineData(2, ApplicationTheme.Dark)]
    public void GetThemeFromSelection_ReturnsTheme(int selection, ApplicationTheme expected)
    {
        Assert.Equal(expected, UiLogic.GetThemeFromSelection(selection));
    }

    [Theory]
    [InlineData(0)]
    [InlineData(-1)]
    [InlineData(3)]
    public void GetThemeFromSelection_ReturnsNullForSystem(int selection)
    {
        Assert.Null(UiLogic.GetThemeFromSelection(selection));
    }

    [Theory]
    [InlineData("Running", false, true)]
    [InlineData("Stopped", true, false)]
    [InlineData("StartPending", false, false)]
    [InlineData("StopPending", false, false)]
    [InlineData("PausePending", false, false)]
    [InlineData("ContinuePending", false, false)]
    [InlineData("Unknown", true, true)]
    public void GetServiceButtonState_MatchesRules(string status, bool startEnabled, bool stopEnabled)
    {
        var state = UiLogic.GetServiceButtonState(status);

        Assert.Equal(startEnabled, state.StartEnabled);
        Assert.Equal(stopEnabled, state.StopEnabled);
    }

    [Theory]
    [InlineData("Running", "Stopped", true)]
    [InlineData("Stopped", "Running", true)]
    [InlineData("StopPending", "Stopped", true)]
    [InlineData("Stopped", "StartPending", true)]
    [InlineData("Stopped", "Stopped", false)]
    [InlineData("Unknown", "Stopped", false)]
    public void ShouldPromptStopServices_TracksServiceStates(string client, string server, bool expected)
    {
        var snapshot = new ServiceStatusSnapshot(client, server);

        Assert.Equal(expected, UiLogic.ShouldPromptStopServices(snapshot));
    }

    [Fact]
    public void BuildTrayTooltip_ReturnsFullTextWhenShort()
    {
        var snapshot = new ServiceStatusSnapshot("Stopped", "Stopped");

        Assert.Equal("Client: Stopped | Server: Stopped", UiLogic.BuildTrayTooltip(snapshot, null));
    }

    [Fact]
    public void BuildTrayTooltip_TruncatesToLimit()
    {
        var client = new string('A', 50);
        var server = new string('B', 50);
        var snapshot = new ServiceStatusSnapshot(client, server);
        var text = $"Client: {client} | Server: {server}";

        var tooltip = UiLogic.BuildTrayTooltip(snapshot, null);

        Assert.Equal(63, tooltip.Length);
        Assert.Equal(text.Substring(0, 63), tooltip);
    }
}
