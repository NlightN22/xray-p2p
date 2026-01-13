param(
    [int] $WinrmPollSeconds = 15,
    [int] $WinrmTimeoutMinutes = 10,
    [string[]] $Machines
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
    & vagrant @Args 2>&1 | ForEach-Object { Write-Host $_ }
    $ErrorActionPreference = $prev
    return $LASTEXITCODE
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

function Ensure-Machine {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Machine
    )

    $state = Get-MachineState -Machine $Machine
    Write-Info ("{0} state: {1}" -f $Machine, $state)

    if ($state -eq "not_created" -or $state -eq "poweroff" -or $state -eq "aborted") {
        Write-Info ("Starting {0} without provision." -f $Machine)
        $exitCode = Invoke-Vagrant -Args @("up", $Machine, "--no-provision")
        if ($exitCode -ne 0) {
            Write-Info ("Initial boot for {0} failed (expected on first boot); continuing to wait." -f $Machine)
        }
    }

    Write-Info ("Waiting for WinRM on {0}." -f $Machine)
    if (-not (Wait-ForWinRM -Machine $Machine)) {
        throw ("Timed out waiting for WinRM on {0} after {1} minutes." -f $Machine, $WinrmTimeoutMinutes)
    }

    Write-Info ("Provisioning {0}." -f $Machine)
    $exitCode = Invoke-Vagrant -Args @("provision", $Machine)
    if ($exitCode -ne 0) {
        throw ("Provision failed for {0} (exit {1})." -f $Machine, $exitCode)
    }
}

Write-Info "Sequential first-boot orchestration started."

$targetMachines = $Machines
if (-not $targetMachines -or $targetMachines.Count -eq 0) {
    $targetMachines = Get-DefinedMachines
}

if (-not $targetMachines -or $targetMachines.Count -eq 0) {
    throw "No Vagrant machines detected. Run this script from a Vagrant directory or pass -Machines."
}

foreach ($machine in $targetMachines) {
    Ensure-Machine -Machine $machine
}

Write-Info "All machines provisioned."
