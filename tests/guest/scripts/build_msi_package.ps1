param(
    [Parameter(Mandatory = $true)]
    [string] $Architecture,

    [Parameter(Mandatory = $true)]
    [string] $CacheDir,

    [Parameter(Mandatory = $true)]
    [string] $WixSource,

    [string] $BuildId,

    [string] $RepoRoot = 'C:\xp2p',
    [string] $Marker = '__MSI_PATH__=',
    [string] $StartMarkerPath = "",
    [string] $DoneMarkerPath = ""
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Write-Stage {
    param([string] $Message)
    $ts = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    Write-Output ("[build_msi_package] {0} {1}" -f $ts, $Message)
}

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
    Write-Stage "Checking MSI dependencies (Go/WiX)"
    if ((Test-GoAvailable) -and (Test-WixAvailable)) {
        Write-Stage "MSI dependencies OK"
        return
    }

    $provisionScript = Join-Path $RepoRoot "infra\vagrant\scripts\win\builder_provision.ps1"
    if (-not (Test-Path $provisionScript)) {
        throw "MSI dependencies missing and provision script not found at $provisionScript."
    }

    $note = "Go/WiX"
    Write-Stage "Installing MSI build dependencies ($note)"
    & $provisionScript 2>&1 | ForEach-Object { Write-Output $_ }
}

$repoRootPath = $RepoRoot
if (-not (Test-Path $repoRootPath)) {
    throw "Shared repo root not found at $repoRootPath. Re-mount the synced folder (try 'vagrant reload --provision')."
}

Write-Stage "Resolve MSI build script for arch=$Architecture"
$scriptInfo = Resolve-MsiScript -Arch $Architecture
$scriptPath = Join-Path $RepoRoot $scriptInfo.Script
if (-not (Test-Path $scriptPath)) {
    throw "MSI build script not found at $scriptPath. Re-mount the synced folder (try 'vagrant reload --provision')."
}

Write-Stage "Resolved MSI script: $scriptPath (archLabel=$($scriptInfo.ArchLabel))"
if ($StartMarkerPath) {
    $startDir = Split-Path -Parent $StartMarkerPath
    if ($startDir) {
        [System.IO.Directory]::CreateDirectory($startDir) | Out-Null
    }
    Set-Content -Path $StartMarkerPath -Value "START" -Encoding ASCII
}
Ensure-MsiDependencies

$buildIdPath = Join-Path $CacheDir 'build-id.txt'
$latestPath = Join-Path $CacheDir 'latest.txt'
if ($BuildId) {
    Write-Stage "Checking MSI cache (BuildId=$BuildId)"
    if ((Test-Path $buildIdPath) -and (Test-Path $latestPath)) {
        $cachedBuildId = (Get-Content -Raw -Path $buildIdPath).Trim()
        if ($cachedBuildId -eq $BuildId) {
            $latest = Get-Content -Path $latestPath -ErrorAction SilentlyContinue
            $msiLine = $latest | Where-Object { $_ -like 'msi_path=*' } | Select-Object -First 1
            if ($msiLine) {
                $cachedPath = $msiLine.Substring('msi_path='.Length).Trim()
                if ($cachedPath -and (Test-Path $cachedPath)) {
                    Write-Stage "Using cached MSI at $cachedPath"
                    Write-Output ("{0}{1}" -f $Marker, $cachedPath)
                    exit 0
                }
            }
        }
    }
}

Write-Stage "Starting MSI build script"
$arguments = @{
    RepoRoot = $RepoRoot
    CacheDir = $CacheDir
    WixSourceRelative = $WixSource
    MsiArchLabel = $scriptInfo.ArchLabel
    OutputMarker = $Marker
    BuildOnly = $true
}

$buildStart = Get-Date
$output = & $scriptPath @arguments 2>&1
$buildEnd = Get-Date
$duration = [math]::Round(($buildEnd - $buildStart).TotalSeconds, 2)
Write-Stage ("MSI build script finished in {0}s" -f $duration)
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

Write-Stage "MSI build output path: $msiPath"
if ($DoneMarkerPath) {
    $doneDir = Split-Path -Parent $DoneMarkerPath
    if ($doneDir) {
        [System.IO.Directory]::CreateDirectory($doneDir) | Out-Null
    }
    Set-Content -Path $DoneMarkerPath -Value $msiPath -Encoding ASCII
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
