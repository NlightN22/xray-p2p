param(
    [Parameter(Mandatory = $true)]
    [string] $Architecture,

    [Parameter(Mandatory = $true)]
    [string] $CacheDir,

    [Parameter(Mandatory = $true)]
    [string] $WixSource,

    [string] $BuildId,

    [string] $RepoRoot = 'C:\xp2p',
    [string] $Marker = '__MSI_PATH__='
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Resolve-MsiScript {
    param([string] $Arch)

    $normalized = $Arch.ToLowerInvariant()
    switch ($normalized) {
        { $_ -in @('amd64', 'x64', 'x86_64') } {
            return @{
                Script = 'scripts\build\build_and_install_msi.ps1'
                ArchLabel = 'amd64'
            }
        }
        { $_ -in @('x86', '386') } {
            return @{
                Script = 'scripts\build\build_and_install_msi_x86.ps1'
                ArchLabel = 'x86'
            }
        }
        default {
            throw "Unsupported architecture '$Arch'. Use 'amd64' or 'x86'."
        }
    }
}

function Test-GoAvailable {
    return $null -ne (Get-Command -Name go.exe -ErrorAction SilentlyContinue)
}

function Test-WixAvailable {
    $wixDirs = Get-ChildItem "C:\Program Files (x86)" -Filter "WiX Toolset*" -Directory -ErrorAction SilentlyContinue
    if (-not $wixDirs) {
        return $false
    }
    $latest = $wixDirs | Sort-Object LastWriteTime -Descending | Select-Object -First 1
    $binPath = Join-Path $latest.FullName "bin"
    return (Test-Path (Join-Path $binPath "candle.exe")) -and (Test-Path (Join-Path $binPath "light.exe"))
}

function Ensure-MsiDependencies {
    if ((Test-GoAvailable) -and (Test-WixAvailable)) {
        return
    }

    $provisionScript = Join-Path $RepoRoot "infra\vagrant\scripts\win\builder_provision.ps1"
    if (-not (Test-Path $provisionScript)) {
        throw "MSI dependencies missing and provision script not found at $provisionScript."
    }

    Write-Host "==> Installing MSI build dependencies (Go/WiX)"
    & $provisionScript 2>&1 | ForEach-Object { Write-Output $_ }
}

$repoRootPath = $RepoRoot
if (-not (Test-Path $repoRootPath)) {
    throw "Shared repo root not found at $repoRootPath. Re-mount the synced folder (try 'vagrant reload --provision')."
}

$scriptInfo = Resolve-MsiScript -Arch $Architecture
$scriptPath = Join-Path $RepoRoot $scriptInfo.Script
if (-not (Test-Path $scriptPath)) {
    throw "MSI build script not found at $scriptPath. Re-mount the synced folder (try 'vagrant reload --provision')."
}

Ensure-MsiDependencies

$buildIdPath = Join-Path $CacheDir 'build-id.txt'
$latestPath = Join-Path $CacheDir 'latest.txt'
if ($BuildId) {
    if ((Test-Path $buildIdPath) -and (Test-Path $latestPath)) {
        $cachedBuildId = (Get-Content -Raw -Path $buildIdPath).Trim()
        if ($cachedBuildId -eq $BuildId) {
            $latest = Get-Content -Path $latestPath -ErrorAction SilentlyContinue
            $msiLine = $latest | Where-Object { $_ -like 'msi_path=*' } | Select-Object -First 1
            if ($msiLine) {
                $cachedPath = $msiLine.Substring('msi_path='.Length).Trim()
                if ($cachedPath -and (Test-Path $cachedPath)) {
                    Write-Output ("{0}{1}" -f $Marker, $cachedPath)
                    exit 0
                }
            }
        }
    }
}

$arguments = @{
    RepoRoot = $RepoRoot
    CacheDir = $CacheDir
    WixSourceRelative = $WixSource
    MsiArchLabel = $scriptInfo.ArchLabel
    OutputMarker = $Marker
    BuildOnly = $true
}

$output = & $scriptPath @arguments 2>&1
$msiPath = $null
foreach ($line in $output) {
    if ($null -eq $line) {
        continue
    }
    $text = $line.ToString().Trim()
    if ($text.StartsWith($Marker)) {
        $msiPath = $text.Substring($Marker.Length).Trim()
    }
    Write-Output $line
}

if (-not $msiPath) {
    throw "MSI build script did not emit marker $Marker"
}
if (-not (Test-Path $msiPath)) {
    throw "MSI package not found at $msiPath"
}

$fileName = [System.IO.Path]::GetFileName($msiPath)
if ($fileName -notmatch '^xp2p-(.+)-windows-') {
    throw "Unable to parse xp2p version from MSI filename '$fileName'"
}
$version = $Matches[1]
$hash = (Get-FileHash -Algorithm SHA256 -Path $msiPath).Hash.ToLowerInvariant()
$latestContent = @(
    "version=$version"
    "sha256=$hash"
    "msi_path=$msiPath"
) -join "`n"
Set-Content -Path $latestPath -Value $latestContent -Encoding ASCII
if ($BuildId) {
    Set-Content -Path $buildIdPath -Value $BuildId -Encoding ASCII
}
