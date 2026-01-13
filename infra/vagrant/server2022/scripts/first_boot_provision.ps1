param(
    [int] $WinrmPollSeconds = 15,
    [int] $WinrmTimeoutMinutes = 60
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
    & vagrant @Args
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

function Wait-ForWinRM {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Machine
    )

    $deadline = (Get-Date).AddMinutes($WinrmTimeoutMinutes)
    while ((Get-Date) -lt $deadline) {
        $exitCode = Invoke-Vagrant -Args @("winrm", $Machine, "-c", "cmd /c echo ready")
        if ($exitCode -eq 0) {
            Write-Info ("WinRM ready for {0}." -f $Machine)
            return $true
        }
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
Ensure-Machine -Machine "win2022-a"
Ensure-Machine -Machine "win2022-b"
Write-Info "All machines provisioned."
