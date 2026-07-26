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

Push-Location $repoRoot
try {
    pytest tests\host\linux\test_resource_plateau.py -vv -s 2>&1 | Tee-Object -FilePath $logPath
    if ($LASTEXITCODE -ne 0) {
        throw "Linux resource plateau suite failed with exit code $LASTEXITCODE. Log: $logPath"
    }
} finally {
    Pop-Location
}

Write-Host "Linux resource plateau suite passed. Log: $logPath"
