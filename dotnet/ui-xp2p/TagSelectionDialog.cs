using System.Collections.Generic;
using System.Linq;
using System.Windows;
using W = System.Windows.Controls;

namespace Xp2pUi;

internal sealed class TagSelectionDialog : Window
{
    private readonly W.ListBox _list;
    private readonly W.Button _ok;

    public TagSelectionDialog(IReadOnlyList<string> tags)
    {
        Title = "Select endpoint tag";
        SizeToContent = SizeToContent.WidthAndHeight;
        WindowStartupLocation = WindowStartupLocation.CenterOwner;
        ResizeMode = ResizeMode.NoResize;
        MinWidth = 320;

        var root = new W.StackPanel { Margin = new Thickness(16) };
        root.Children.Add(new W.TextBlock
        {
            Text = "Select an endpoint tag for full-tunnel mode:",
            Margin = new Thickness(0, 0, 0, 8)
        });

        _list = new W.ListBox
        {
            ItemsSource = tags.ToList(),
            MinHeight = 120,
            MinWidth = 260
        };
        root.Children.Add(_list);

        var buttons = new W.StackPanel
        {
            Orientation = W.Orientation.Horizontal,
            HorizontalAlignment = System.Windows.HorizontalAlignment.Right,
            Margin = new Thickness(0, 12, 0, 0)
        };
        var ok = new W.Button { Content = "OK", Width = 80, IsEnabled = false };
        _ok = ok;
        var cancel = new W.Button { Content = "Cancel", Width = 80, Margin = new Thickness(8, 0, 0, 0) };
        _ok.Click += (_, _) => Accept();
        cancel.Click += (_, _) => Close();
        buttons.Children.Add(_ok);
        buttons.Children.Add(cancel);
        root.Children.Add(buttons);

        Content = root;

        _list.SelectionChanged += (_, _) => ok.IsEnabled = _list.SelectedItem is not null;
    }

    public string? SelectedTag { get; private set; }

    private void Accept()
    {
        SelectedTag = _list.SelectedItem as string;
        if (!string.IsNullOrWhiteSpace(SelectedTag))
        {
            DialogResult = true;
        }
    }
}
