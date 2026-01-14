<#
.SYNOPSIS
Installs the xp2p MSI if latest.txt differs from the recorded install state.

.PARAMETER LatestPath
Path to latest.txt with version/sha256/msi_path entries.

.PARAMETER Force
Reinstall even if latest.txt matches the local state file.

.PARAMETER StatePath
Local state file used to compare version and sha256.
#>
param(
    [string] $LatestPath = 'C:\xp2p\build\msi-artifacts\latest.txt',
    [bool] $Force = $false,
    [string] $StatePath = 'C:\ProgramData\xp2p\msi-install-state.txt'
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Read-KeyValueFile {
    param([string] $Path)

    if (-not (Test-Path $Path)) {
        return $null
    }

    $content = Get-Content -Raw -Path $Path
    $data = @{}
    foreach ($line in ($content -split "`r?`n")) {
        $trimmed = $line.Trim()
        if (-not $trimmed) {
            continue
        }
        $parts = $trimmed.Split("=", 2)
        if ($parts.Count -ne 2) {
            throw "Invalid line in $Path: $trimmed"
        }
        $data[$parts[0].Trim().ToLowerInvariant()] = $parts[1].Trim()
    }
    return $data
}

function Ensure-Directory {
    param([string] $Path)
    if (-not (Test-Path $Path)) {
        New-Item -ItemType Directory -Path $Path -Force | Out-Null
    }
}

$latest = Read-KeyValueFile -Path $LatestPath
if (-not $latest) {
    throw "latest.txt not found at $LatestPath"
}

$version = $latest["version"]
$sha256 = $latest["sha256"]
$msiPath = $latest["msi_path"]
if (-not $version -or -not $sha256 -or -not $msiPath) {
    throw "latest.txt is missing required fields (version, sha256, msi_path)"
}
if (-not (Test-Path $msiPath)) {
    throw "MSI package not found at $msiPath"
}

$state = Read-KeyValueFile -Path $StatePath
$needsInstall = $Force -or -not $state
if (-not $needsInstall) {
    if ($state["version"] -ne $version -or $state["sha256"] -ne $sha256) {
        $needsInstall = $true
    }
}

$xp2pExe = Join-Path $env:ProgramFiles 'xp2p\xp2p.exe'
if (-not (Test-Path $xp2pExe)) {
    $needsInstall = $true
}

if (-not $needsInstall) {
    Write-Output "__MSI_INSTALL_SKIP__"
    exit 0
}

$arguments = @('/i', $msiPath, '/qn', '/norestart', 'XP2P_SKIP_SERVICE_START=1')
$process = Start-Process -FilePath 'msiexec.exe' -ArgumentList $arguments -PassThru
if (-not $process.WaitForExit(300000)) {
    Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
    exit 124
}
if ($process.ExitCode -ne 0) {
    exit $process.ExitCode
}

$stateDir = Split-Path -Parent $StatePath
if ($stateDir) {
    Ensure-Directory -Path $stateDir
}
$stateContent = @(
    "version=$version"
    "sha256=$sha256"
    "msi_path=$msiPath"
) -join "`n"
Set-Content -Path $StatePath -Value $stateContent -Encoding ASCII
exit 0
