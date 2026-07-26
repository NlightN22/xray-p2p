param(
    [ValidateSet("quick", "nightly")]
    [string] $Profile = "quick"
)

$ErrorActionPreference = "Stop"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$logRoot = Join-Path $repoRoot ".logs\tests"
New-Item -ItemType Directory -Force -Path $logRoot | Out-Null
$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$logPath = Join-Path $logRoot "pytest-linux-resource-plateau-$Profile-$stamp.log"

$env:XP2P_RUN_RESOURCE_PLATEAU = "1"
$env:XP2P_RESOURCE_PLATEAU_PROFILE = $Profile
$env:XP2P_RUN_HEARTBEAT_STORM_TESTS = "1"
$testTargets = @(
    "tests\host\linux\test_resource_plateau.py",
    "tests\host\linux\test_resource_plateau_faults.py",
    "tests\host\linux\test_network_lifecycle_shutdown.py::test_control_server_listener_closes_before_service_stop_returns",
    "tests\host\linux\test_network_lifecycle_shutdown.py::test_running_xray_exits_before_service_stop_returns"
)

Push-Location $repoRoot
try {
    pytest @testTargets -vv -s 2>&1 | Tee-Object -FilePath $logPath
    if ($LASTEXITCODE -ne 0) {
        throw "Linux resource plateau suite failed with exit code $LASTEXITCODE. Log: $logPath"
    }
} finally {
    Pop-Location
}

Write-Host "Linux resource plateau suite passed. Log: $logPath"
