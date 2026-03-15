using System;
using System.Diagnostics;
using System.IO;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Interop;
using System.Windows.Media;
using System.Windows.Media.Imaging;
using Application = System.Windows.Application;
using Forms = System.Windows.Forms;

namespace Xp2pUi;

internal sealed class App : Application
{
    private Forms.NotifyIcon? _trayIcon;
    private MainWindow? _window;

    [STAThread]
    public static void Main()
    {
        var app = new App { ShutdownMode = ShutdownMode.OnExplicitShutdown };
        app.Startup += app.OnStartup;
        app.Exit += app.OnExit;
        app.Run();
    }

    private void OnStartup(object? sender, StartupEventArgs e)
    {
        Log("xp2p-ui starting.");
        var appIcon = GetAppIcon();
        _window = new MainWindow(CreateIconSource(appIcon));
        _window.Hide();

        _trayIcon = new Forms.NotifyIcon
        {
            Icon = appIcon,
            Text = "xp2p",
            Visible = true,
            ContextMenuStrip = BuildMenu()
        };
        _trayIcon.DoubleClick += (_, _) => ShowWindow("Ready.");
    }

    private void OnExit(object? sender, ExitEventArgs e)
    {
        Log("xp2p-ui exiting.");
        if (_trayIcon is not null)
        {
            _trayIcon.Visible = false;
            _trayIcon.Dispose();
        }
    }

    private Forms.ContextMenuStrip BuildMenu()
    {
        var clientMenu = new Forms.ToolStripMenuItem("Client");
        clientMenu.DropDownItems.Add(new Forms.ToolStripMenuItem("Start", null, (_, _) => ShowWindow("Client start requested.")));
        clientMenu.DropDownItems.Add(new Forms.ToolStripMenuItem("Stop", null, (_, _) => ShowWindow("Client stop requested.")));
        clientMenu.DropDownItems.Add(new Forms.ToolStripMenuItem("Install", null, (_, _) => ShowWindow("Client install requested.")));
        clientMenu.DropDownItems.Add(new Forms.ToolStripMenuItem("Deploy", null, (_, _) => ShowWindow("Client deploy requested.")));

        var serverMenu = new Forms.ToolStripMenuItem("Server");
        serverMenu.DropDownItems.Add(new Forms.ToolStripMenuItem("Start", null, (_, _) => ShowWindow("Server start requested.")));
        serverMenu.DropDownItems.Add(new Forms.ToolStripMenuItem("Stop", null, (_, _) => ShowWindow("Server stop requested.")));
        serverMenu.DropDownItems.Add(new Forms.ToolStripMenuItem("Install", null, (_, _) => ShowWindow("Server install requested.")));
        serverMenu.DropDownItems.Add(new Forms.ToolStripMenuItem("Deploy", null, (_, _) => ShowWindow("Server deploy requested.")));

        var openLogs = new Forms.ToolStripMenuItem("Open logs", null, (_, _) => OpenLogs());
        var quit = new Forms.ToolStripMenuItem("Quit", null, (_, _) => Shutdown());

        var menu = new Forms.ContextMenuStrip();
        menu.Items.Add(clientMenu);
        menu.Items.Add(serverMenu);
        menu.Items.Add(new Forms.ToolStripSeparator());
        menu.Items.Add(openLogs);
        menu.Items.Add(new Forms.ToolStripSeparator());
        menu.Items.Add(quit);
        return menu;
    }

    private void ShowWindow(string message)
    {
        if (_window is null)
        {
            return;
        }
        Dispatcher.Invoke(() =>
        {
            _window.SetStatus(message);
            _window.Show();
            _window.Activate();
        });
    }

    private void OpenLogs()
    {
        var logPath = GetLogPath();
        if (!File.Exists(logPath))
        {
            ShowWindow($"Log file not found: {logPath}");
            return;
        }

        try
        {
            Process.Start(new ProcessStartInfo
            {
                FileName = "explorer.exe",
                Arguments = $"/select,\"{logPath}\"",
                UseShellExecute = true
            });
            ShowWindow("Log file opened.");
        }
        catch (Exception ex)
        {
            ShowWindow($"Open log failed: {ex.Message}");
        }
    }

    private static string GetLogPath()
    {
        var programData = Environment.GetFolderPath(Environment.SpecialFolder.CommonApplicationData);
        if (string.IsNullOrWhiteSpace(programData))
        {
            programData = @"C:\ProgramData";
        }
        return Path.Combine(programData, "xp2p", "logs", "xp2p-ui.log");
    }

    private void Log(string message)
    {
        try
        {
            var logPath = GetLogPath();
            var logDir = Path.GetDirectoryName(logPath);
            if (!string.IsNullOrWhiteSpace(logDir))
            {
                Directory.CreateDirectory(logDir);
            }
            var line = $"{DateTime.UtcNow:O} INFO xp2p: {message}";
            File.AppendAllText(logPath, line + Environment.NewLine);
        }
        catch
        {
            // Avoid crashing on log failures.
        }
    }

    private static System.Drawing.Icon GetAppIcon()
    {
        var exePath = Process.GetCurrentProcess().MainModule?.FileName;
        if (!string.IsNullOrWhiteSpace(exePath))
        {
            try
            {
                var icon = System.Drawing.Icon.ExtractAssociatedIcon(exePath);
                if (icon is not null)
                {
                    return icon;
                }
            }
            catch
            {
                // Fall back to the default icon.
            }
        }
        return System.Drawing.SystemIcons.Application;
    }

    private static ImageSource? CreateIconSource(System.Drawing.Icon icon)
    {
        try
        {
            return Imaging.CreateBitmapSourceFromHIcon(
                icon.Handle,
                Int32Rect.Empty,
                BitmapSizeOptions.FromEmptyOptions());
        }
        catch
        {
            return null;
        }
    }
}

internal sealed class MainWindow : Window
{
    private readonly TextBlock _status;

    public MainWindow(ImageSource? icon)
    {
        Title = "xp2p UI (WPF)";
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

        var root = new StackPanel
        {
            Margin = new Thickness(16),
            Orientation = System.Windows.Controls.Orientation.Vertical
        };
        root.Children.Add(new TextBlock
        {
            Text = "xp2p UI (WPF)",
            FontSize = 16,
            Margin = new Thickness(0, 0, 0, 12)
        });
        _status = new TextBlock
        {
            Text = "Ready.",
            Margin = new Thickness(0, 0, 0, 12)
        };
        root.Children.Add(_status);

        var hide = new System.Windows.Controls.Button { Content = "Hide window", Width = 120 };
        hide.Click += (_, _) => Hide();
        root.Children.Add(hide);

        Content = root;
    }

    public void SetStatus(string message)
    {
        _status.Text = message;
    }
}
