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

& $scriptPath @arguments
