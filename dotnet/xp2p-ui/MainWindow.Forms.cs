using System;
using System.Collections.Generic;
using System.Windows;
using W = System.Windows.Controls;
using M = System.Windows.Media;

namespace Xp2pUi;

internal sealed partial class MainWindow
{
    private UIElement BuildClientInstallTab()
    {
        var tabs = new W.TabControl();
        var linkTab = new W.TabItem { Header = "Install by link" };
        linkTab.Content = BuildClientInstallByLink();
        tabs.Items.Add(linkTab);

        var manualTab = new W.TabItem { Header = "Manual setup" };
        manualTab.Content = BuildClientInstallManual();
        tabs.Items.Add(manualTab);

        return tabs;
    }

    private UIElement BuildClientInstallByLink()
    {
        var panel = new W.StackPanel { Margin = new Thickness(0, 6, 0, 0) };
        var required = new List<RequiredField>();

        var link = CreateTextField(panel, "Link *", required: true, requiredFields: required);
        var installDir = CreateTextField(panel, "Install dir", required: false, requiredFields: required);
        var configDir = CreateTextField(panel, "Config dir", required: false, requiredFields: required);

        var run = new W.Button { Content = "Install client", Width = 140, Margin = new Thickness(0, 12, 0, 0) };
        run.Click += (_, _) =>
        {
            if (!ValidateRequired(required))
            {
                SetStatus("Fill in required fields.");
                return;
            }
            var request = new ClientInstallRequest(
                UseLink: true,
                Link: link.Text,
                InstallDir: installDir.Text,
                ConfigDir: configDir.Text,
                Host: "",
                Port: "",
                User: "",
                Password: "",
                ServerName: "",
                AllowInsecure: false,
                StrictTls: false);
            var result = _backend.ClientInstall(request);
            SetStatus(result.Message);
        };
        panel.Children.Add(run);
        return new W.ScrollViewer { Content = panel };
    }

    private UIElement BuildClientInstallManual()
    {
        var panel = new W.StackPanel { Margin = new Thickness(0, 6, 0, 0) };
        var required = new List<RequiredField>();

        var installDir = CreateTextField(panel, "Install dir", required: false, requiredFields: required);
        var configDir = CreateTextField(panel, "Config dir", required: false, requiredFields: required);
        var host = CreateTextField(panel, "Host *", required: true, requiredFields: required);
        var port = CreateTextField(panel, "Port", required: false, requiredFields: required);
        var user = CreateTextField(panel, "User *", required: true, requiredFields: required);
        var password = CreatePasswordField(panel, "Password *", required: true, requiredFields: required);
        var serverName = CreateTextField(panel, "Server name (SNI)", required: false, requiredFields: required);
        var allowInsecure = CreateCheckBox(panel, "Allow insecure TLS");
        var strictTls = CreateCheckBox(panel, "Strict TLS");

        var run = new W.Button { Content = "Install client", Width = 140, Margin = new Thickness(0, 12, 0, 0) };
        run.Click += (_, _) =>
        {
            if (!ValidateRequired(required))
            {
                SetStatus("Fill in required fields.");
                return;
            }
            var request = new ClientInstallRequest(
                UseLink: false,
                Link: "",
                InstallDir: installDir.Text,
                ConfigDir: configDir.Text,
                Host: host.Text,
                Port: port.Text,
                User: user.Text,
                Password: password.Password,
                ServerName: serverName.Text,
                AllowInsecure: allowInsecure.IsChecked == true,
                StrictTls: strictTls.IsChecked == true);
            var result = _backend.ClientInstall(request);
            SetStatus(result.Message);
        };
        panel.Children.Add(run);
        return new W.ScrollViewer { Content = panel };
    }

    private UIElement BuildClientDeployTab()
    {
        var panel = new W.StackPanel { Margin = new Thickness(0, 6, 0, 0) };
        var required = new List<RequiredField>();

        var host = CreateTextField(panel, "Host *", required: true, requiredFields: required);
        var port = CreateTextField(panel, "Deploy port", required: false, requiredFields: required);
        var installDir = CreateTextField(panel, "Install dir", required: false, requiredFields: required);
        var user = CreateTextField(panel, "User", required: false, requiredFields: required);
        var password = CreatePasswordField(panel, "Password", required: false, requiredFields: required);
        var trojanPort = CreateTextField(panel, "Trojan port", required: false, requiredFields: required);

        var run = new W.Button { Content = "Deploy client", Width = 140, Margin = new Thickness(0, 12, 0, 0) };
        run.Click += (_, _) =>
        {
            if (!ValidateRequired(required))
            {
                SetStatus("Fill in required fields.");
                return;
            }
            var request = new ClientDeployRequest(
                Host: host.Text,
                Port: port.Text,
                InstallDir: installDir.Text,
                User: user.Text,
                Password: password.Password,
                TrojanPort: trojanPort.Text);
            var result = _backend.ClientDeploy(request);
            SetStatus(result.Message);
        };
        panel.Children.Add(run);
        return new W.ScrollViewer { Content = panel };
    }

    private UIElement BuildServerInstallTab()
    {
        var panel = new W.StackPanel { Margin = new Thickness(0, 6, 0, 0) };
        var required = new List<RequiredField>();

        var path = CreateTextField(panel, "Install dir", required: false, requiredFields: required);
        var configDir = CreateTextField(panel, "Config dir", required: false, requiredFields: required);
        var port = CreateTextField(panel, "Port", required: false, requiredFields: required);
        var certStore = CreateTextField(panel, "Cert store", required: false, requiredFields: required);
        var certFile = CreateTextField(panel, "Cert file", required: false, requiredFields: required);
        var keyFile = CreateTextField(panel, "Key file", required: false, requiredFields: required);
        var host = CreateTextField(panel, "Host", required: false, requiredFields: required);

        var run = new W.Button { Content = "Install server", Width = 140, Margin = new Thickness(0, 12, 0, 0) };
        run.Click += (_, _) =>
        {
            var request = new ServerInstallRequest(
                Path: path.Text,
                ConfigDir: configDir.Text,
                Port: port.Text,
                CertStore: certStore.Text,
                CertFile: certFile.Text,
                KeyFile: keyFile.Text,
                Host: host.Text);
            var result = _backend.ServerInstall(request);
            SetStatus(result.Message);
        };
        panel.Children.Add(run);
        return new W.ScrollViewer { Content = panel };
    }

    private UIElement BuildServerDeployTab()
    {
        var panel = new W.StackPanel { Margin = new Thickness(0, 6, 0, 0) };
        var required = new List<RequiredField>();

        var listen = CreateTextField(panel, "Listen", required: false, requiredFields: required);
        var link = CreateTextField(panel, "Link *", required: true, requiredFields: required);
        var diagPort = CreateTextField(panel, "Diag port", required: false, requiredFields: required);
        var timeout = CreateTextField(panel, "Timeout", required: false, requiredFields: required);

        var run = new W.Button { Content = "Deploy server", Width = 140, Margin = new Thickness(0, 12, 0, 0) };
        run.Click += (_, _) =>
        {
            if (!ValidateRequired(required))
            {
                SetStatus("Fill in required fields.");
                return;
            }
            var request = new ServerDeployRequest(
                Listen: listen.Text,
                Link: link.Text,
                DiagPort: diagPort.Text,
                Timeout: timeout.Text);
            var result = _backend.ServerDeploy(request);
            SetStatus(result.Message);
        };
        panel.Children.Add(run);
        return new W.ScrollViewer { Content = panel };
    }

    private static W.TextBox CreateTextField(
        W.Panel parent,
        string label,
        bool required,
        List<RequiredField> requiredFields)
    {
        var field = new W.TextBox { MinWidth = 240 };
        AddLabeledField(parent, label, field);
        if (required)
        {
            requiredFields.Add(new RequiredField(field, () => field.Text));
        }
        return field;
    }

    private static W.PasswordBox CreatePasswordField(
        W.Panel parent,
        string label,
        bool required,
        List<RequiredField> requiredFields)
    {
        var field = new W.PasswordBox { MinWidth = 240 };
        AddLabeledField(parent, label, field);
        if (required)
        {
            requiredFields.Add(new RequiredField(field, () => field.Password));
        }
        return field;
    }

    private static W.CheckBox CreateCheckBox(W.Panel parent, string label)
    {
        var checkbox = new W.CheckBox
        {
            Content = label,
            Margin = new Thickness(0, 6, 0, 0)
        };
        parent.Children.Add(checkbox);
        return checkbox;
    }

    private static void AddLabeledField(W.Panel parent, string label, W.Control field)
    {
        var wrapper = new W.StackPanel { Margin = new Thickness(0, 0, 0, 6) };
        wrapper.Children.Add(new W.TextBlock { Text = label });
        wrapper.Children.Add(field);
        parent.Children.Add(wrapper);
    }

    private static bool ValidateRequired(List<RequiredField> fields)
    {
        var ok = true;
        foreach (var field in fields)
        {
            var value = field.ValueProvider();
            var isEmpty = string.IsNullOrWhiteSpace(value);
            if (field.Control.Tag is not M.Brush original)
            {
                field.Control.Tag = field.Control.BorderBrush;
                original = field.Control.BorderBrush;
            }
            field.Control.BorderBrush = isEmpty ? M.Brushes.IndianRed : original;
            if (isEmpty)
            {
                ok = false;
            }
        }
        return ok;
    }

    private sealed class RequiredField
    {
        public RequiredField(W.Control control, Func<string> valueProvider)
        {
            Control = control;
            ValueProvider = valueProvider;
        }

        public W.Control Control { get; }
        public Func<string> ValueProvider { get; }
    }
}
