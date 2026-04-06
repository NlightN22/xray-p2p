using System;
using System.Collections.Generic;
using System.Windows;
using System.Windows.Media;
using ModernWpf;
using W = System.Windows.Controls;

namespace Xp2pUi;

internal sealed partial class MainWindow : Window
{
    private readonly IBackend _backend;
    private readonly ServiceManager _serviceManager;
    private readonly W.TextBlock _status;
    private readonly W.TabControl _tabs;
    private readonly Dictionary<TabKey, W.TabItem> _tabMap;
    private W.TextBlock? _clientStatus;
    private W.TextBlock? _serverStatus;
    private W.TextBlock? _clientRuntime;
    private W.TextBlock? _clientMode;
    private W.TextBlock? _serverMode;
    private W.Button? _clientModeButton;
    private W.Button? _serverModeButton;
    private ClientMode? _clientModeValue;
    private ServerMode? _serverModeValue;
    private bool _clientModePending;
    private bool _serverModePending;
    private readonly Dictionary<string, (W.Button Start, W.Button Stop, W.Button Restart)> _serviceButtons = new();

    public MainWindow(IBackend backend, ServiceManager serviceManager, ImageSource? icon)
    {
        _backend = backend;
        _serviceManager = serviceManager;
        Title = "xp2p UI";
        Width = 640;
        Height = 520;
        MinWidth = 640;
        MinHeight = 520;
        if (icon is not null)
        {
            Icon = icon;
        }
        Closing += (_, e) =>
        {
            e.Cancel = true;
            Hide();
        };

        var root = new W.Grid { Margin = new Thickness(16) };
        root.RowDefinitions.Add(new W.RowDefinition { Height = GridLength.Auto });
        root.RowDefinitions.Add(new W.RowDefinition { Height = new GridLength(1, GridUnitType.Star) });
        root.RowDefinitions.Add(new W.RowDefinition { Height = GridLength.Auto });

        var headerRow = new W.Grid { Margin = new Thickness(0, 0, 0, 12) };
        headerRow.ColumnDefinitions.Add(new W.ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
        headerRow.ColumnDefinitions.Add(new W.ColumnDefinition { Width = GridLength.Auto });

        var header = new W.TextBlock
        {
            Text = "xp2p UI",
            FontSize = 16
        };
        W.Grid.SetColumn(header, 0);
        headerRow.Children.Add(header);

        var themeSelector = new W.ComboBox
        {
            Width = 120,
            ItemsSource = new[] { "System", "Light", "Dark" },
            SelectedIndex = 0
        };
        themeSelector.SelectionChanged += (_, _) => ApplyTheme(themeSelector.SelectedIndex);
        W.Grid.SetColumn(themeSelector, 1);
        headerRow.Children.Add(themeSelector);

        W.Grid.SetRow(headerRow, 0);
        root.Children.Add(headerRow);

        _tabs = new W.TabControl();
        _tabMap = BuildTabs();
        W.Grid.SetRow(_tabs, 1);
        root.Children.Add(_tabs);

        var statusPanel = new W.StackPanel
        {
            Orientation = W.Orientation.Vertical,
            Margin = new Thickness(0, 12, 0, 0)
        };
        statusPanel.Children.Add(new W.TextBlock
        {
            Text = "Status",
            FontWeight = FontWeights.SemiBold,
            Margin = new Thickness(0, 0, 0, 4)
        });
        _status = new W.TextBlock { Text = "Ready." };
        statusPanel.Children.Add(_status);
        var hide = new W.Button
        {
            Content = "Hide window",
            Width = 120,
            Margin = new Thickness(0, 10, 0, 0)
        };
        hide.Click += (_, _) => Hide();
        statusPanel.Children.Add(hide);

        W.Grid.SetRow(statusPanel, 2);
        root.Children.Add(statusPanel);

        Content = root;
    }

    public event EventHandler<ClientMode>? ClientModeRequested;
    public event EventHandler<ServerMode>? ServerModeRequested;

    public void SetStatus(string message)
    {
        _status.Text = message;
    }

    public void SelectTab(TabKey tabKey)
    {
        if (_tabMap.TryGetValue(tabKey, out var tab))
        {
            _tabs.SelectedItem = tab;
        }
    }

    private Dictionary<TabKey, W.TabItem> BuildTabs()
    {
        var map = new Dictionary<TabKey, W.TabItem>();

        var statusTab = new W.TabItem
        {
            Header = "Status",
            Content = BuildStatusTab()
        };
        _tabs.Items.Add(statusTab);
        map[TabKey.Status] = statusTab;

        return map;
    }

    private UIElement BuildStatusTab()
    {
        var panel = new W.StackPanel { Margin = new Thickness(0, 6, 0, 0) };
        panel.Children.Add(new W.TextBlock
        {
            Text = "Service status",
            FontWeight = FontWeights.SemiBold,
            Margin = new Thickness(0, 0, 0, 6)
        });
        var clientStatus = _serviceManager.GetStatus(ServiceNames.Client);
        var serverStatus = _serviceManager.GetStatus(ServiceNames.Server);
        _clientStatus = new W.TextBlock { Text = $"Client: {clientStatus}" };
        _serverStatus = new W.TextBlock { Text = $"Server: {serverStatus}" };
        panel.Children.Add(BuildServiceRow(ServiceNames.Client, _clientStatus));
        panel.Children.Add(BuildServiceRow(ServiceNames.Server, _serverStatus));
        _clientRuntime = new W.TextBlock { Text = "Tun: Unknown" };
        panel.Children.Add(_clientRuntime);

        _clientMode = new W.TextBlock { Text = "Client mode: Unknown" };
        _clientModeButton = new W.Button { Content = "Mode", Width = 80, Margin = new Thickness(4, 0, 0, 0) };
        _clientModeButton.Click += (_, _) => ShowClientModeMenu();
        panel.Children.Add(BuildModeRow(_clientMode, _clientModeButton));

        _serverMode = new W.TextBlock { Text = "Server mode: Unknown" };
        _serverModeButton = new W.Button { Content = "Mode", Width = 80, Margin = new Thickness(4, 0, 0, 0) };
        _serverModeButton.Click += (_, _) => ShowServerModeMenu();
        panel.Children.Add(BuildModeRow(_serverMode, _serverModeButton));

        UpdateServiceButtons(ServiceNames.Client, clientStatus);
        UpdateServiceButtons(ServiceNames.Server, serverStatus);

        var refresh = new W.Button { Content = "Refresh", Width = 120, Margin = new Thickness(0, 10, 0, 0) };
        refresh.Click += (_, _) =>
        {
            RefreshServiceStatusLabels();
            SetStatus("Service status refreshed.");
        };
        panel.Children.Add(refresh);
        return new W.ScrollViewer { Content = panel };
    }

    private UIElement BuildServiceRow(string serviceName, W.TextBlock status)
    {
        var row = new W.StackPanel
        {
            Orientation = W.Orientation.Horizontal,
            Margin = new Thickness(0, 0, 0, 8)
        };
        status.Width = 220;
        row.Children.Add(status);

        var start = new W.Button { Content = "Start", Width = 70, Margin = new Thickness(4, 0, 0, 0) };
        var stop = new W.Button { Content = "Stop", Width = 70, Margin = new Thickness(6, 0, 0, 0) };
        var restart = new W.Button { Content = "Restart", Width = 80, Margin = new Thickness(6, 0, 0, 0) };
        start.Click += async (_, _) =>
        {
            ToggleServiceButtons(start, stop, restart, false);
            SetStatus(await _serviceManager.StartServiceAsync(serviceName));
            RefreshServiceStatusLabels();
            ToggleServiceButtons(start, stop, restart, true);
        };
        stop.Click += async (_, _) =>
        {
            ToggleServiceButtons(start, stop, restart, false);
            SetStatus(await _serviceManager.StopServiceAsync(serviceName));
            RefreshServiceStatusLabels();
            ToggleServiceButtons(start, stop, restart, true);
        };
        restart.Click += async (_, _) =>
        {
            ToggleServiceButtons(start, stop, restart, false);
            SetStatus(await _serviceManager.RestartServiceAsync(serviceName));
            RefreshServiceStatusLabels();
            ToggleServiceButtons(start, stop, restart, true);
        };
        row.Children.Add(start);
        row.Children.Add(stop);
        row.Children.Add(restart);
        _serviceButtons[serviceName] = (start, stop, restart);

        return row;
    }

    private static UIElement BuildModeRow(W.TextBlock status, W.Button button)
    {
        var row = new W.StackPanel
        {
            Orientation = W.Orientation.Horizontal,
            Margin = new Thickness(0, 0, 0, 8)
        };
        status.Width = 220;
        row.Children.Add(status);
        row.Children.Add(button);
        return row;
    }

    private void RefreshServiceStatusLabels()
    {
        if (_clientStatus is not null)
        {
            var status = _serviceManager.GetStatus(ServiceNames.Client);
            _clientStatus.Text = $"Client: {status}";
            UpdateServiceButtons(ServiceNames.Client, status);
        }
        if (_serverStatus is not null)
        {
            var status = _serviceManager.GetStatus(ServiceNames.Server);
            _serverStatus.Text = $"Server: {status}";
            UpdateServiceButtons(ServiceNames.Server, status);
        }
    }

    public void SetClientRuntimeStatus(string message)
    {
        if (_clientRuntime is null)
        {
            return;
        }
        _clientRuntime.Text = message;
    }

    public void SetClientModeStatus(ClientMode? mode, string label, bool pending)
    {
        _clientModeValue = mode;
        _clientModePending = pending;
        if (_clientMode is not null)
        {
            _clientMode.Text = $"Client mode: {label}";
        }
        if (_clientModeButton is not null)
        {
            _clientModeButton.IsEnabled = !pending;
        }
    }

    public void SetServerModeStatus(ServerMode? mode, string label, bool pending)
    {
        _serverModeValue = mode;
        _serverModePending = pending;
        if (_serverMode is not null)
        {
            _serverMode.Text = $"Server mode: {label}";
        }
        if (_serverModeButton is not null)
        {
            _serverModeButton.IsEnabled = !pending;
        }
    }

    private static void ToggleServiceButtons(W.Button start, W.Button stop, W.Button restart, bool enabled)
    {
        start.IsEnabled = enabled;
        stop.IsEnabled = enabled;
        restart.IsEnabled = enabled;
    }

    private void UpdateServiceButtons(string serviceName, string status)
    {
        if (!_serviceButtons.TryGetValue(serviceName, out var buttons))
        {
            return;
        }
        var state = UiLogic.GetServiceButtonState(status);
        buttons.Start.IsEnabled = state.StartEnabled;
        buttons.Stop.IsEnabled = state.StopEnabled;
        buttons.Restart.IsEnabled = UiLogic.IsRestartEnabled(status);
    }

    private static void ApplyTheme(int selection)
    {
        try
        {
            ThemeManager.Current.ApplicationTheme = UiLogic.GetThemeFromSelection(selection);
        }
        catch
        {
            // Ignore theme update failures to keep the UI responsive.
        }
    }

    private void ShowClientModeMenu()
    {
        if (_clientModeButton is null || _clientModePending)
        {
            return;
        }
        var menu = new W.ContextMenu();
        foreach (var option in ModeLogic.ClientAlternatives(_clientModeValue))
        {
            var label = ModeLogic.FormatClientMode(option);
            var item = new W.MenuItem { Header = label };
            item.Click += (_, _) => ClientModeRequested?.Invoke(this, option);
            menu.Items.Add(item);
        }
        _clientModeButton.ContextMenu = menu;
        menu.IsOpen = true;
    }

    private void ShowServerModeMenu()
    {
        if (_serverModeButton is null || _serverModePending)
        {
            return;
        }
        var menu = new W.ContextMenu();
        foreach (var option in ModeLogic.ServerAlternatives(_serverModeValue))
        {
            var label = ModeLogic.FormatServerMode(option);
            var item = new W.MenuItem { Header = label };
            item.Click += (_, _) => ServerModeRequested?.Invoke(this, option);
            menu.Items.Add(item);
        }
        menu.Items.Add(new W.Separator());
        menu.Items.Add(new W.MenuItem
        {
            Header = "Split/Full modes are not supported on server.",
            IsEnabled = false
        });
        _serverModeButton.ContextMenu = menu;
        menu.IsOpen = true;
    }
}
