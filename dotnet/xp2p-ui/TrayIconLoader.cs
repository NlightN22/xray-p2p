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
    public static TrayIconSet Load(System.Drawing.Icon fallback, Action<string>? log = null)
    {
        var baseIcon = LoadIconFromResource("assets/Disabled.png", fallback, log);
        var enabledIcon = LoadIconFromResource("assets/Enabled.png", fallback, log);
        var busyIcon = LoadIconFromResource("assets/Enabling.png", fallback, log);
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

    private static System.Drawing.Icon LoadIconFromResource(string resourcePath, System.Drawing.Icon fallback, Action<string>? log)
    {
        if (TryLoadFromManifest(resourcePath, log, out var icon))
        {
            return icon;
        }
        if (TryLoadFromFile(resourcePath, log, out icon))
        {
            return icon;
        }
        log?.Invoke($"tray icon resource failed: {resourcePath} (fallback)");
        return CloneIcon(fallback);
    }


    private static bool TryLoadFromFile(string resourcePath, Action<string>? log, out System.Drawing.Icon icon)
    {
        try
        {
            var baseDir = AppContext.BaseDirectory;
            var cleaned = resourcePath.TrimStart('/').Replace('/', Path.DirectorySeparatorChar);
            var path = Path.Combine(baseDir, cleaned);
            if (!File.Exists(path))
            {
                log?.Invoke($"tray icon file missing: {path}");
                icon = null!;
                return false;
            }
            using var stream = File.OpenRead(path);
            icon = CreateIconFromPng(stream);
            log?.Invoke($"tray icon file loaded: {path}");
            return true;
        }
        catch (Exception ex)
        {
            log?.Invoke($"tray icon file failed: {resourcePath} error={ex.GetType().Name} {ex.Message}");
            icon = null!;
            return false;
        }
    }

    private static bool TryLoadFromManifest(string resourcePath, Action<string>? log, out System.Drawing.Icon icon)
    {
        try
        {
            var assembly = typeof(TrayIconLoader).Assembly;
            var suffix = resourcePath.TrimStart('/').Replace('/', '.');
            var matched = false;
            foreach (var name in assembly.GetManifestResourceNames())
            {
                if (!name.EndsWith(suffix, StringComparison.OrdinalIgnoreCase))
                {
                    continue;
                }
                matched = true;
                using var stream = assembly.GetManifestResourceStream(name);
                if (stream is null)
                {
                    continue;
                }
                icon = CreateIconFromPng(stream);
                log?.Invoke($"tray icon manifest loaded: {name}");
                return true;
            }
            if (!matched)
            {
                log?.Invoke($"tray icon manifest missing: *{suffix}");
            }
        }
        catch (Exception ex)
        {
            log?.Invoke($"tray icon manifest failed: {resourcePath} error={ex.GetType().Name} {ex.Message}");
            // Ignore and fall back.
        }
        icon = null!;
        return false;
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
