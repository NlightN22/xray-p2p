param()

$ErrorActionPreference = "Stop"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$logRoot = Join-Path $repoRoot ".logs\tests"
New-Item -ItemType Directory -Force -Path $logRoot | Out-Null
$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$logPath = Join-Path $logRoot "pytest-linux-all-opt-in-$stamp.log"

$env:XP2P_RUN_SERVICE_CLI_TESTS = "1"
$env:XP2P_RUN_MANUAL_EDIT_TESTS = "1"
$env:XP2P_RUN_HEARTBEAT_STORM_TESTS = "1"
$env:XP2P_RUN_DESTRUCTIVE_TESTS = "1"
$env:XP2P_RUN_DUAL_DEPLOY_TESTS = "1"
$env:XP2P_RUN_EXTERNAL_SUBSCRIPTION_MATRIX = "1"
$env:XP2P_RUN_RESOURCE_PLATEAU = "1"
$env:XP2P_RESOURCE_PLATEAU_PROFILE = "nightly"

Push-Location $repoRoot
try {
    $pytestOutput = @(pytest tests\host\linux -vv -s 2>&1 | Tee-Object -FilePath $logPath)
    if ($LASTEXITCODE -ne 0) {
        throw "Linux release suite failed with exit code $LASTEXITCODE. Log: $logPath"
    }
    if (($pytestOutput -join "`n") -notmatch "\b[1-9][0-9]* passed\b") {
        throw "Linux release suite did not execute any passing tests. Log: $logPath"
    }
} finally {
    Pop-Location
}

Write-Host "Linux release suite passed. Log: $logPath"
