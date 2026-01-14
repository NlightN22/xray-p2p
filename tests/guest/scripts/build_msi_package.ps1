param(
    [Parameter(Mandatory = $true)]
    [string] $Architecture,

    [Parameter(Mandatory = $true)]
    [string] $CacheDir,

    [Parameter(Mandatory = $true)]
    [string] $WixSource,

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

$scriptInfo = Resolve-MsiScript -Arch $Architecture
$scriptPath = Join-Path $RepoRoot $scriptInfo.Script
if (-not (Test-Path $scriptPath)) {
    throw "MSI build script not found at $scriptPath"
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
$latestPath = Join-Path $CacheDir 'latest.txt'
$latestContent = @(
    "version=$version"
    "sha256=$hash"
    "msi_path=$msiPath"
) -join "`n"
Set-Content -Path $latestPath -Value $latestContent -Encoding ASCII
