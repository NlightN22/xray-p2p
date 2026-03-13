param(
    [int] $WinrmPollSeconds = 15,
    [int] $WinrmTimeoutMinutes = 10,
    [string[]] $Machines,
    [string] $Machine
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Write-Info {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Message
    )
    Write-Host "==> $Message"
}

function Invoke-Vagrant {
    param(
        [Parameter(Mandatory = $true)]
        [string[]] $Args
    )
    Write-Info ("Running: vagrant {0}" -f ($Args -join " "))
    $prev = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $output = & vagrant @Args 2>&1
    $output | ForEach-Object { Write-Host $_ }
    $ErrorActionPreference = $prev
    return [pscustomobject]@{
        ExitCode = $LASTEXITCODE
        Output = ($output -join "`n")
    }
}

function Test-HostOnlyMissingError {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Output
    )
    return $Output -match "VERR_INTNET_FLT_IF_NOT_FOUND"
}

function Test-VboxNameCollisionError {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Output
    )
    return ($Output -match "VERR_ALREADY_EXISTS" -and $Output -match "Could not rename the directory")
}

function Get-VboxNameCollisionPaths {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Output
    )
    $match = [regex]::Match($Output, "Could not rename the directory '([^']+)' to '([^']+)'")
    if (-not $match.Success) {
        return $null
    }
    return [pscustomobject]@{
        From = $match.Groups[1].Value
        To   = $match.Groups[2].Value
    }
}

function Get-RegisteredVmNames {
    $names = @()
    $lines = & VBoxManage list vms 2>$null
    foreach ($line in $lines) {
        if ($line -match '^"([^"]+)"') {
            $names += $matches[1]
        }
    }
    return $names
}

function Try-FixVboxNameCollision {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Machine,
        [Parameter(Mandatory = $true)]
        [string] $Output
    )
    if (-not (Test-VboxNameCollisionError -Output $Output)) {
        return $false
    }
    $paths = Get-VboxNameCollisionPaths -Output $Output
    if (-not $paths) {
        return $false
    }
    $registered = Get-RegisteredVmNames
    if ($registered -contains $Machine) {
        Write-Info ("VirtualBox already has a VM named {0}; skip folder cleanup." -f $Machine)
        return $false
    }
    if (-not [string]::IsNullOrWhiteSpace($paths.To) -and (Test-Path -LiteralPath $paths.To)) {
        Write-Info ("Removing stale VM folder for {0}: {1}" -f $Machine, $paths.To)
        Remove-Item -LiteralPath $paths.To -Recurse -Force -ErrorAction SilentlyContinue
        return $true
    }
    return $false
}

function Get-MachineState {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Machine
    )

    $lines = & vagrant status --machine-readable
    foreach ($line in $lines) {
        $parts = $line -split ",", 4
        if ($parts.Length -lt 4) {
            continue
        }
        if ($parts[1] -ne $Machine) {
            continue
        }
        if ($parts[2] -eq "state") {
            return $parts[3]
        }
    }
    return "unknown"
}

function Get-DefinedMachines {
    $lines = & vagrant status --machine-readable
    $machines = @()
    foreach ($line in $lines) {
        $parts = $line -split ",", 4
        if ($parts.Length -lt 4) {
            continue
        }
        if ($parts[2] -ne "state") {
            continue
        }
        if (-not [string]::IsNullOrWhiteSpace($parts[1])) {
            $machines += $parts[1]
        }
    }

    return $machines | Sort-Object -Unique
}

function Wait-ForWinRM {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Machine
    )

    $deadline = (Get-Date).AddMinutes($WinrmTimeoutMinutes)
    $attempt = 0
    while ((Get-Date) -lt $deadline) {
        $attempt++
        Write-Info ("Running: vagrant winrm {0} -c 'cmd /c echo ready'" -f $Machine)
        $prev = $ErrorActionPreference
        $ErrorActionPreference = "Continue"
        $output = & vagrant winrm $Machine -c "cmd /c echo ready" 2>&1
        $output | ForEach-Object { Write-Host $_ }
        $ErrorActionPreference = $prev
        $exitCode = $LASTEXITCODE
        if ($exitCode -eq 0 -and ($output -match "ready")) {
            Write-Info ("WinRM ready for {0}." -f $Machine)
            return $true
        }
        Write-Info ("WinRM probe {0} for {1} failed (exit {2})." -f $attempt, $Machine, $exitCode)
        Start-Sleep -Seconds $WinrmPollSeconds
    }
    return $false
}

function Test-SyncedFolder {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Machine,
        [string] $Path = "C:\\xp2p"
    )

    Write-Info ("Checking synced folder on {0}: {1}" -f $Machine, $Path)
    $prev = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $escaped = $Path.Replace("'", "''")
    $psScript = "if (Test-Path '$escaped') { Write-Output 'sync-ok'; exit 0 } else { Write-Output 'sync-missing'; exit 3 }"
    $bytes = [System.Text.Encoding]::Unicode.GetBytes($psScript)
    $encoded = [System.Convert]::ToBase64String($bytes)
    $cmd = "powershell -NoProfile -EncodedCommand $encoded"
    $output = & vagrant winrm $Machine -c $cmd 2>&1
    $output | ForEach-Object { Write-Host $_ }
    $ErrorActionPreference = $prev
    return $LASTEXITCODE -eq 0
}

function Ensure-Machine {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Machine
    )

    $state = Get-MachineState -Machine $Machine
    Write-Info ("{0} state: {1}" -f $Machine, $state)

    if ($state -eq "not_created" -or $state -eq "poweroff" -or $state -eq "aborted") {
        Write-Info ("Starting {0} without provision." -f $Machine)
        $result = Invoke-Vagrant -Args @("up", $Machine, "--no-provision")
        if ($result.ExitCode -ne 0) {
            if (Test-HostOnlyMissingError -Output $result.Output) {
                throw ("VirtualBox host-only adapter not found while starting {0}. " +
                    "Create a host-only interface (VBoxManage hostonlyif create) and retry." -f $Machine)
            }
            if (Try-FixVboxNameCollision -Machine $Machine -Output $result.Output) {
                Write-Info ("Retrying {0} after name-collision cleanup." -f $Machine)
                $retry = Invoke-Vagrant -Args @("up", $Machine, "--no-provision")
                if ($retry.ExitCode -ne 0) {
                    Write-Info ("Retry boot for {0} failed (exit {1}); continuing to wait." -f $Machine, $retry.ExitCode)
                }
            }
            Write-Info ("Initial boot for {0} failed (expected on first boot); continuing to wait." -f $Machine)
        }
    }

    Write-Info ("Waiting for WinRM on {0}." -f $Machine)
    if (-not (Wait-ForWinRM -Machine $Machine)) {
        throw ("Timed out waiting for WinRM on {0} after {1} minutes." -f $Machine, $WinrmTimeoutMinutes)
    }

    if (-not (Test-SyncedFolder -Machine $Machine)) {
        Write-Info ("Synced folder missing on {0}; reloading with provision." -f $Machine)
        $result = Invoke-Vagrant -Args @("reload", $Machine, "--provision", "--force")
        if ($result.ExitCode -ne 0) {
            if (Test-HostOnlyMissingError -Output $result.Output) {
                throw ("VirtualBox host-only adapter not found while reloading {0}. " +
                    "Create a host-only interface (VBoxManage hostonlyif create) and retry." -f $Machine)
            }
            Write-Info ("Reload/provision failed for {0} (exit {1}); continuing with manual wait/provision." -f $Machine, $result.ExitCode)
        }
        Write-Info ("Waiting for WinRM on {0} after reload." -f $Machine)
        if (-not (Wait-ForWinRM -Machine $Machine)) {
            throw ("Timed out waiting for WinRM on {0} after reload." -f $Machine)
        }
        if (-not (Test-SyncedFolder -Machine $Machine)) {
            throw ("Synced folder still missing on {0} after reload/provision." -f $Machine)
        }
    }

    Write-Info ("Provisioning {0}." -f $Machine)
    $result = Invoke-Vagrant -Args @("provision", $Machine)
    if ($result.ExitCode -ne 0) {
        throw ("Provision failed for {0} (exit {1})." -f $Machine, $result.ExitCode)
    }
}

Write-Info "Sequential first-boot orchestration started."

$targetMachines = $Machines
if ($Machine) {
    $targetMachines = @($Machine)
}
if ($targetMachines -and $targetMachines -is [string]) {
    $targetMachines = @($targetMachines)
}

if (-not $targetMachines -or ($targetMachines | Measure-Object).Count -eq 0) {
    $targetMachines = Get-DefinedMachines
}

if (-not $targetMachines -or ($targetMachines | Measure-Object).Count -eq 0) {
    throw "No Vagrant machines detected. Run this script from a Vagrant directory or pass -Machines."
}

foreach ($machine in $targetMachines) {
    Ensure-Machine -Machine $machine
}

Write-Info "All machines provisioned."
