using System;

namespace Xp2pUi;

internal static class Program
{
    [STAThread]
    public static void Main()
    {
        var app = new App();
        app.Run();
    }
}
