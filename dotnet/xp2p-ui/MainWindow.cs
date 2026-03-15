using System;
using System.Collections.Generic;
using System.Windows;
using System.Windows.Media;
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

        var header = new W.TextBlock
        {
            Text = "xp2p UI",
            FontSize = 16,
            Margin = new Thickness(0, 0, 0, 12)
        };
        W.Grid.SetRow(header, 0);
        root.Children.Add(header);

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

        var clientInstallTab = new W.TabItem
        {
            Header = "Client install",
            Content = BuildClientInstallTab()
        };
        _tabs.Items.Add(clientInstallTab);
        map[TabKey.ClientInstall] = clientInstallTab;

        var clientDeployTab = new W.TabItem
        {
            Header = "Client deploy",
            Content = BuildClientDeployTab()
        };
        _tabs.Items.Add(clientDeployTab);
        map[TabKey.ClientDeploy] = clientDeployTab;

        var serverInstallTab = new W.TabItem
        {
            Header = "Server install",
            Content = BuildServerInstallTab()
        };
        _tabs.Items.Add(serverInstallTab);
        map[TabKey.ServerInstall] = serverInstallTab;

        var serverDeployTab = new W.TabItem
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
        var panel = new W.StackPanel { Margin = new Thickness(0, 6, 0, 0) };
        panel.Children.Add(new W.TextBlock
        {
            Text = "Service status",
            FontWeight = FontWeights.SemiBold,
            Margin = new Thickness(0, 0, 0, 6)
        });
        _clientStatus = new W.TextBlock { Text = $"Client: {_serviceManager.GetStatus(ServiceNames.Client)}" };
        _serverStatus = new W.TextBlock { Text = $"Server: {_serviceManager.GetStatus(ServiceNames.Server)}" };
        panel.Children.Add(BuildServiceRow(ServiceNames.Client, _clientStatus));
        panel.Children.Add(BuildServiceRow(ServiceNames.Server, _serverStatus));

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
        start.Click += (_, _) =>
        {
            SetStatus(_serviceManager.StartService(serviceName));
            RefreshServiceStatusLabels();
        };
        row.Children.Add(start);

        var stop = new W.Button { Content = "Stop", Width = 70, Margin = new Thickness(6, 0, 0, 0) };
        stop.Click += (_, _) =>
        {
            SetStatus(_serviceManager.StopService(serviceName));
            RefreshServiceStatusLabels();
        };
        row.Children.Add(stop);

        return row;
    }

    private void RefreshServiceStatusLabels()
    {
        if (_clientStatus is not null)
        {
            _clientStatus.Text = $"Client: {_serviceManager.GetStatus(ServiceNames.Client)}";
        }
        if (_serverStatus is not null)
        {
            _serverStatus.Text = $"Server: {_serviceManager.GetStatus(ServiceNames.Server)}";
        }
    }
}
