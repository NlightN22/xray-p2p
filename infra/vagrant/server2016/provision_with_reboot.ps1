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
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $psi.EnvironmentVariables["VAGRANT_CWD"] = $scriptRoot

    Write-Host ("==> Running: {0} {1}" -f $psi.FileName, $psi.Arguments)
    Write-Host ("==> WorkingDirectory: {0}" -f $psi.WorkingDirectory)

    $proc = New-Object System.Diagnostics.Process
    $proc.StartInfo = $psi
    [void]$proc.Start()
    $proc.WaitForExit()

    $stdOut = $proc.StandardOutput.ReadToEnd()
    $stdErr = $proc.StandardError.ReadToEnd()
    if ($stdOut) {
        Write-Host $stdOut
    }
    if ($stdErr) {
        Write-Host $stdErr
    }

    Write-Host ("==> ExitCode: {0}" -f $proc.ExitCode)

    return @{
        ExitCode = $proc.ExitCode
        Output   = ($stdOut + "`n" + $stdErr).Trim()
    }
}

function Get-VagrantState {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Machine
    )

    $vagrantCmd = Get-Command -Name "vagrant" -ErrorAction SilentlyContinue
    if (-not $vagrantCmd) {
        throw "vagrant not found in PATH. Ensure Vagrant is installed and available."
    }

    $env:VAGRANT_CWD = $scriptRoot
    $output = & $vagrantCmd.Source status $Machine --machine-readable 2>&1
    if (-not $output) {
        return $null
    }

    foreach ($line in $output) {
        if ($line -match ",state,([^,]+)$") {
            return $Matches[1]
        }
    }

    return $null
}

function Ensure-MachineUp {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Machine
    )

    $state = Get-VagrantState -Machine $Machine
    if (-not $state) {
        Write-Host ("==> Unable to determine state for {0}; running 'up'." -f $Machine)
        $result = Invoke-Vagrant -Arguments ("up {0} --provision" -f $Machine)
        if ($result.ExitCode -ne 0) {
            Write-Host ("==> Initial provisioning for {0} failed. Attempting reboot + provision retry." -f $Machine)
            $reload = Invoke-Vagrant -Arguments ("reload {0} --force" -f $Machine)
            if ($reload.ExitCode -ne 0) {
                exit $reload.ExitCode
            }
            $retry = Invoke-Vagrant -Arguments ("provision {0}" -f $Machine)
            if ($retry.ExitCode -ne 0) {
                exit $retry.ExitCode
            }
        }
        return
    }

    if ($state -in @("not_created", "poweroff", "saved", "aborted", "stopped")) {
        Write-Host ("==> {0} state is {1}; running 'up --provision'." -f $Machine, $state)
        $result = Invoke-Vagrant -Arguments ("up {0} --provision" -f $Machine)
        if ($result.ExitCode -ne 0) {
            Write-Host ("==> Initial provisioning for {0} failed. Attempting reboot + provision retry." -f $Machine)
            $reload = Invoke-Vagrant -Arguments ("reload {0} --force" -f $Machine)
            if ($reload.ExitCode -ne 0) {
                exit $reload.ExitCode
            }
            $retry = Invoke-Vagrant -Arguments ("provision {0}" -f $Machine)
            if ($retry.ExitCode -ne 0) {
                exit $retry.ExitCode
            }
        }
    }
    else {
        Write-Host ("==> {0} state is {1}; skipping 'up'." -f $Machine, $state)
    }
}

function Needs-Reboot {
    param(
        [AllowEmptyString()]
        [string] $Output = ""
    )

    if ([string]::IsNullOrWhiteSpace($Output)) {
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

Ensure-MachineUp -Machine "win2016-a"
Ensure-MachineUp -Machine "win2016-b"

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
