using System;
using System.Collections.Generic;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Media;

namespace Xp2pUi;

internal sealed partial class MainWindow : Window
{
    private readonly IBackend _backend;
    private readonly ServiceManager _serviceManager;
    private readonly TextBlock _status;
    private readonly TabControl _tabs;
    private readonly Dictionary<TabKey, TabItem> _tabMap;

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

        var root = new Grid { Margin = new Thickness(16) };
        root.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });
        root.RowDefinitions.Add(new RowDefinition { Height = new GridLength(1, GridUnitType.Star) });
        root.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });

        var header = new TextBlock
        {
            Text = "xp2p UI",
            FontSize = 16,
            Margin = new Thickness(0, 0, 0, 12)
        };
        Grid.SetRow(header, 0);
        root.Children.Add(header);

        _tabs = new TabControl();
        _tabMap = BuildTabs();
        Grid.SetRow(_tabs, 1);
        root.Children.Add(_tabs);

        var statusPanel = new StackPanel
        {
            Orientation = Orientation.Vertical,
            Margin = new Thickness(0, 12, 0, 0)
        };
        statusPanel.Children.Add(new TextBlock
        {
            Text = "Status",
            FontWeight = FontWeights.SemiBold,
            Margin = new Thickness(0, 0, 0, 4)
        });
        _status = new TextBlock { Text = "Ready." };
        statusPanel.Children.Add(_status);
        var hide = new Button
        {
            Content = "Hide window",
            Width = 120,
            Margin = new Thickness(0, 10, 0, 0)
        };
        hide.Click += (_, _) => Hide();
        statusPanel.Children.Add(hide);

        Grid.SetRow(statusPanel, 2);
        root.Children.Add(statusPanel);

        Content = root;
    }

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

    private Dictionary<TabKey, TabItem> BuildTabs()
    {
        var map = new Dictionary<TabKey, TabItem>();

        var statusTab = new TabItem
        {
            Header = "Status",
            Content = BuildStatusTab()
        };
        _tabs.Items.Add(statusTab);
        map[TabKey.Status] = statusTab;

        var clientInstallTab = new TabItem
        {
            Header = "Client install",
            Content = BuildClientInstallTab()
        };
        _tabs.Items.Add(clientInstallTab);
        map[TabKey.ClientInstall] = clientInstallTab;

        var clientDeployTab = new TabItem
        {
            Header = "Client deploy",
            Content = BuildClientDeployTab()
        };
        _tabs.Items.Add(clientDeployTab);
        map[TabKey.ClientDeploy] = clientDeployTab;

        var serverInstallTab = new TabItem
        {
            Header = "Server install",
            Content = BuildServerInstallTab()
        };
        _tabs.Items.Add(serverInstallTab);
        map[TabKey.ServerInstall] = serverInstallTab;

        var serverDeployTab = new TabItem
        {
            Header = "Server deploy",
            Content = BuildServerDeployTab()
        };
        _tabs.Items.Add(serverDeployTab);
        map[TabKey.ServerDeploy] = serverDeployTab;

        return map;
    }

    private UIElement BuildStatusTab()
    {
        var panel = new StackPanel { Margin = new Thickness(0, 6, 0, 0) };
        panel.Children.Add(new TextBlock
        {
            Text = "Service status",
            FontWeight = FontWeights.SemiBold,
            Margin = new Thickness(0, 0, 0, 6)
        });
        var clientStatus = new TextBlock { Text = $"Client: {_serviceManager.GetStatus(ServiceNames.Client)}" };
        var serverStatus = new TextBlock { Text = $"Server: {_serviceManager.GetStatus(ServiceNames.Server)}" };
        panel.Children.Add(clientStatus);
        panel.Children.Add(serverStatus);

        var refresh = new Button
        {
            Content = "Refresh",
            Width = 120,
            Margin = new Thickness(0, 10, 0, 0)
        };
        refresh.Click += (_, _) =>
        {
            clientStatus.Text = $"Client: {_serviceManager.GetStatus(ServiceNames.Client)}";
            serverStatus.Text = $"Server: {_serviceManager.GetStatus(ServiceNames.Server)}";
            SetStatus("Service status refreshed.");
        };
        panel.Children.Add(refresh);
        return new ScrollViewer { Content = panel };
    }
}
