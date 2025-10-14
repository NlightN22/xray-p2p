[CmdletBinding()]
param(
    [switch]$KeepEnvironment,
    [ValidateSet('start','ssh','prechecks','r2','r3','reverse')]
    [string]$StartFrom = 'start',
    [switch]$VerboseLogs
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$libPath = [System.IO.Path]::Combine($PSScriptRoot, 'lib.ps1')
. $libPath

Set-VerboseLogs -Enabled:$VerboseLogs.IsPresent
Reset-TestResults

$stageSequence = @('start', 'ssh', 'prechecks', 'r2', 'r3', 'reverse')
$stageIndex = @{}
for ($i = 0; $i -lt $stageSequence.Count; $i++) {
    $stageIndex[$stageSequence[$i]] = $i
}
$startStageIndex = $stageIndex[$StartFrom]

function ShouldRunStage {
    param([string]$Stage)
    return $stageIndex[$Stage] -ge $startStageIndex
}

$destroyOnExit = -not $KeepEnvironment
$testsPassed = $false

$c1Ip = $null
$c2Ip = $null
$c3Ip = $null

try {
    if (ShouldRunStage 'start') {
        Start-VagrantEnvironment -DestroyBetweenAttempts:$destroyOnExit
    }
    else {
        Write-Step "Skipping Vagrant bootstrap (StartFrom=$StartFrom)."
    }

    if (ShouldRunStage 'ssh') {
        Test-SshReadiness
    }
    else {
        Write-Step "Skipping SSH readiness checks (StartFrom=$StartFrom)."
    }

    if (ShouldRunStage 'prechecks') {
        Run-Prechecks
    }
    else {
        Write-Step "Skipping pre-setup connectivity checks (StartFrom=$StartFrom)."
    }

    if (ShouldRunStage 'r2') {
        $stageR2 = Invoke-StageR2
        $c1Ip = $stageR2.C1Ip
    }
    else {
        Write-Step "Skipping r2 provisioning (StartFrom=$StartFrom)."
        if (ShouldRunStage 'r3' -or ShouldRunStage 'reverse') {
            $c1Ip = Get-InterfaceIpv4 -Machine 'c1' -Interface 'eth1'
            Write-Detail "Detected c1 eth1 address for reuse: $c1Ip"
        }
    }

    if (ShouldRunStage 'r3') {
        $stageR3 = Invoke-StageR3 -C1Ip $c1Ip
        $c1Ip = $stageR3.C1Ip
        $c2Ip = $stageR3.C2Ip
        $c3Ip = $stageR3.C3Ip
    }
    else {
        Write-Step "Skipping r3 provisioning (StartFrom=$StartFrom)."
        if (ShouldRunStage 'reverse') {
            if (-not $c1Ip) {
                $c1Ip = Get-InterfaceIpv4 -Machine 'c1' -Interface 'eth1'
                Write-Detail "Detected c1 eth1 address for reverse checks: $c1Ip"
            }
            if (-not $c2Ip) {
                $c2Ip = Get-InterfaceIpv4 -Machine 'c2' -Interface 'eth1'
                Write-Detail "Detected c2 eth1 address: $c2Ip"
            }
            if (-not $c3Ip) {
                $c3Ip = Get-InterfaceIpv4 -Machine 'c3' -Interface 'eth1'
                Write-Detail "Detected c3 eth1 address: $c3Ip"
            }
        }
    }

    if (ShouldRunStage 'reverse') {
        if (-not $c1Ip) {
            $c1Ip = Get-InterfaceIpv4 -Machine 'c1' -Interface 'eth1'
            Write-Detail "Detected c1 eth1 address for reverse checks: $c1Ip"
        }
        if (-not $c2Ip) {
            $c2Ip = Get-InterfaceIpv4 -Machine 'c2' -Interface 'eth1'
            Write-Detail "Detected c2 eth1 address: $c2Ip"
        }
        if (-not $c3Ip) {
            $c3Ip = Get-InterfaceIpv4 -Machine 'c3' -Interface 'eth1'
            Write-Detail "Detected c3 eth1 address: $c3Ip"
        }
        Test-ReverseTunnels -C1Ip $c1Ip -C2Ip $c2Ip -C3Ip $c3Ip
    }
    else {
        Write-Step "Skipping reverse tunnel validation (StartFrom=$StartFrom)."
    }
    $results = Get-TestResults
    $testsPassed = ($results | Where-Object { $_.Status -eq 'FAIL' }).Count -eq 0
    if ($testsPassed) {
        Write-Step "All connectivity checks succeeded"
    }
    else {
        Write-Step "Connectivity checks encountered failures"
    }
}
finally {
    if ($destroyOnExit) {
        if ($testsPassed) {
            Write-Step "Destroying Vagrant environment"
            Invoke-Vagrant -Arguments @('destroy', '-f')
        }
        else {
            Write-Step "Skipping destroy to preserve environment for debugging (rerun with -KeepEnvironment to keep it explicitly)."
        }
    }
    else {
        Write-Step "Leaving Vagrant environment running (KeepEnvironment requested)"
    }
}

Publish-TestResults | Out-Null

if (-not $testsPassed) {
    throw "One or more connectivity checks failed."
}
