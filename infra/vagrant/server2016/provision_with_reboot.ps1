$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$scriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$scriptRoot = (Resolve-Path $scriptRoot).Path
$env:VAGRANT_CWD = $scriptRoot

function Invoke-Vagrant {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Arguments
    )

    $vagrantCmd = Get-Command -Name "vagrant" -ErrorAction SilentlyContinue
    if (-not $vagrantCmd) {
        throw "vagrant not found in PATH. Ensure Vagrant is installed and available."
    }

    $psi = [System.Diagnostics.ProcessStartInfo]::new()
    $psi.FileName = $vagrantCmd.Source
    $psi.Arguments = $Arguments
    $psi.WorkingDirectory = $scriptRoot
    $psi.UseShellExecute = $false
    $psi.CreateNoWindow = $false
    $psi.EnvironmentVariables["VAGRANT_CWD"] = $scriptRoot

    Write-Host ("==> Running: {0} {1}" -f $psi.FileName, $psi.Arguments)
    Write-Host ("==> WorkingDirectory: {0}" -f $psi.WorkingDirectory)

    $proc = New-Object System.Diagnostics.Process
    $proc.StartInfo = $psi
    [void]$proc.Start()
    $proc.WaitForExit()

    Write-Host ("==> ExitCode: {0}" -f $proc.ExitCode)

    return @{
        ExitCode = $proc.ExitCode
        Output   = ""
    }
}

function Needs-Reboot {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Output
    )

    if (-not $Output) {
        return $false
    }

    $patterns = @(
        "reboot is required",
        "restart required",
        "OpenSSH capability install requires a reboot",
        "RestartNeeded"
    )

    foreach ($pattern in $patterns) {
        if ($Output -match [regex]::Escape($pattern)) {
            return $true
        }
    }

    return $false
}

$firstRun = Invoke-Vagrant -Arguments "reload --provision --force"
if ($firstRun.ExitCode -eq 0) {
    exit 0
}

if (-not (Needs-Reboot -Output $firstRun.Output)) {
    exit $firstRun.ExitCode
}

Write-Host "==> Reboot requested by provisioning output. Reloading VM and re-provisioning."
$reload = Invoke-Vagrant -Arguments "reload --force"
if ($reload.ExitCode -ne 0) {
    exit $reload.ExitCode
}

$secondRun = Invoke-Vagrant -Arguments "provision"
exit $secondRun.ExitCode
