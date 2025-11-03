param(
    [Parameter(Mandatory = $true)]
    [ValidateSet('server', 'client')]
    [string] $Role
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Write-Info {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Message
    )

    Write-Host "==> $Message"
}

$source = 'C:\xp2p\build\windows-amd64\xp2p.exe'
$installRoot = 'C:\Program Files\xp2p'
$binDir = Join-Path $installRoot 'bin'

if (-not (Test-Path $source)) {
    throw "xp2p build binary not found at $source"
}

Write-Info "Preparing Program Files install for role '$Role'"

if (Test-Path $installRoot) {
    Write-Info "Removing existing install directory $installRoot"
    try {
        Remove-Item $installRoot -Recurse -Force -ErrorAction Stop
    }
    catch {
        Remove-Item $installRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
}

Write-Info "Creating bin directory $binDir"
New-Item -ItemType Directory -Path $binDir -Force | Out-Null

$target = Join-Path $binDir 'xp2p.exe'
Write-Info "Copying xp2p.exe to $target"
Copy-Item $source $target -Force

Write-Info "Granting Modify rights to vagrant on $installRoot"
icacls $installRoot /grant 'vagrant:(OI)(CI)M' /t /c | Out-Null

Write-Info "Program Files xp2p preparation complete."
