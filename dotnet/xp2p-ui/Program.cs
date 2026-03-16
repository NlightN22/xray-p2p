using System;
using System.Threading;

namespace Xp2pUi;

internal static class Program
{
    [STAThread]
    public static void Main()
    {
        using var mutex = new Mutex(true, @"Global\xp2p-ui", out var createdNew);
        if (!createdNew)
        {
            return;
        }
        var app = new App();
        app.Run();
    }
}
