using System;
using System.IO;
using System.Runtime.InteropServices;
using System.Windows;
using System.Windows.Interop;
using System.Windows.Media;
using System.Windows.Media.Imaging;

namespace Xp2pUi;

internal sealed class TrayIconSet : IDisposable
{
    public TrayIconSet(System.Drawing.Icon baseIcon, System.Drawing.Icon enabled, System.Drawing.Icon busy)
    {
        Base = baseIcon;
        Enabled = enabled;
        Busy = busy;
    }

    public System.Drawing.Icon Base { get; }
    public System.Drawing.Icon Enabled { get; }
    public System.Drawing.Icon Busy { get; }

    public void Dispose()
    {
        Base.Dispose();
        Enabled.Dispose();
        Busy.Dispose();
    }
}

internal static class TrayIconLoader
{
    public static TrayIconSet Load(System.Drawing.Icon fallback)
    {
        var baseIcon = LoadIconFromResource("assets/Base.png", fallback);
        var enabledIcon = LoadIconFromResource("assets/Enabled.png", fallback);
        var busyIcon = LoadIconFromResource("assets/Enabling.png", fallback);
        return new TrayIconSet(baseIcon, enabledIcon, busyIcon);
    }

    public static ImageSource? CreateIconSource(System.Drawing.Icon icon)
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

    private static System.Drawing.Icon LoadIconFromResource(string resourcePath, System.Drawing.Icon fallback)
    {
        try
        {
            var info = System.Windows.Application.GetResourceStream(BuildPackUri(resourcePath));
            if (info is null)
            {
                return CloneIcon(fallback);
            }
            using var stream = info.Stream;
            return CreateIconFromPng(stream);
        }
        catch
        {
            return CloneIcon(fallback);
        }
    }

    private static Uri BuildPackUri(string resourcePath)
    {
        var assemblyName = typeof(TrayIconLoader).Assembly.GetName().Name ?? "Xp2pUi";
        var cleaned = resourcePath.TrimStart('/');
        return new Uri($"pack://application:,,,/{assemblyName};component/{cleaned}", UriKind.Absolute);
    }

    private static System.Drawing.Icon CreateIconFromPng(Stream stream)
    {
        using var bitmap = new System.Drawing.Bitmap(stream);
        var hIcon = bitmap.GetHicon();
        try
        {
            return (System.Drawing.Icon)System.Drawing.Icon.FromHandle(hIcon).Clone();
        }
        finally
        {
            DestroyIcon(hIcon);
        }
    }

    private static System.Drawing.Icon CloneIcon(System.Drawing.Icon icon)
    {
        return (System.Drawing.Icon)icon.Clone();
    }

    [DllImport("user32.dll", SetLastError = true)]
    private static extern bool DestroyIcon(IntPtr hIcon);
}
