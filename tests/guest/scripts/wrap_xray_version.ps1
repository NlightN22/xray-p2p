param(
    [Parameter(Mandatory = $true)]
    [string] $XrayPath,

    [Parameter(Mandatory = $true)]
    [string] $BackupPath,

    [Parameter(Mandatory = $true)]
    [string] $FakeVersion
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

if (-not (Test-Path $XrayPath)) {
    throw "xray binary not found at $XrayPath"
}

$backupDir = Split-Path -Parent $BackupPath
if ($backupDir -and -not (Test-Path $backupDir)) {
    New-Item -ItemType Directory -Path $backupDir -Force | Out-Null
}

if (-not (Test-Path $BackupPath)) {
    Move-Item -Path $XrayPath -Destination $BackupPath -Force
}

$safeBackup = $BackupPath.Replace('\', '\\').Replace('"', '\"')
$safeVersion = $FakeVersion.Replace('\', '\\').Replace('"', '\"')

$source = @"
using System;
using System.Diagnostics;
using System.Linq;
using System.Text;

class Program {
    static int Main(string[] args) {
        if (args.Length > 0 && (args[0] == "-version" || args[0] == "--version")) {
            Console.WriteLine("Xray $safeVersion (xp2p test wrapper)");
            return 0;
        }

        var psi = new ProcessStartInfo();
        psi.FileName = "$safeBackup";
        psi.Arguments = string.Join(" ", args.Select(QuoteArgument));
        psi.UseShellExecute = false;
        var proc = Process.Start(psi);
        if (proc == null) {
            Console.Error.WriteLine("Failed to start real xray process.");
            return 3;
        }
        proc.WaitForExit();
        return proc.ExitCode;
    }

    static string QuoteArgument(string arg) {
        if (string.IsNullOrEmpty(arg)) {
            return "\"\"";
        }
        bool needsQuotes = arg.Any(ch => char.IsWhiteSpace(ch) || ch == '"');
        if (!needsQuotes) {
            return arg;
        }
        var sb = new StringBuilder();
        sb.Append('"');
        int backslashes = 0;
        foreach (var ch in arg) {
            if (ch == '\\') {
                backslashes++;
                continue;
            }
            if (ch == '"') {
                sb.Append(new string('\\', backslashes * 2 + 1));
                sb.Append('"');
                backslashes = 0;
                continue;
            }
            if (backslashes > 0) {
                sb.Append(new string('\\', backslashes));
                backslashes = 0;
            }
            sb.Append(ch);
        }
        if (backslashes > 0) {
            sb.Append(new string('\\', backslashes * 2));
        }
        sb.Append('"');
        return sb.ToString();
    }
}
"@

Add-Type -TypeDefinition $source -Language CSharp -OutputAssembly $XrayPath -OutputType ConsoleApplication | Out-Null
