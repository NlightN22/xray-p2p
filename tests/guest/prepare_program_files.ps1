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
$logsDir = Join-Path $installRoot 'logs'

if (-not (Test-Path $source)) {
    throw "xp2p build binary not found at $source"
}

Write-Info "Preparing Program Files install for role '$Role'"

if (Test-Path $installRoot) {
    Write-Info "Removing existing install directory $installRoot"
    $xp2pProcs = Get-Process -Name xp2p -ErrorAction SilentlyContinue
    if ($xp2pProcs) {
        Write-Info "Stopping running xp2p processes"
        foreach ($proc in $xp2pProcs) {
            try {
                Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
            }
            catch {
                Write-Info ("Failed to stop xp2p process Id={0}: {1}" -f $proc.Id, $_.Exception.Message)
            }
        }
        Start-Sleep -Seconds 1
    }
    try {
        Remove-Item $installRoot -Recurse -Force -ErrorAction Stop
    }
    catch {
        Remove-Item $installRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
}

Write-Info "Creating install directory $installRoot"
New-Item -ItemType Directory -Path $installRoot -Force | Out-Null

Write-Info "Creating bin directory $binDir"
New-Item -ItemType Directory -Path $binDir -Force | Out-Null

Write-Info "Creating logs directory $logsDir"
New-Item -ItemType Directory -Path $logsDir -Force | Out-Null

$xp2pTarget = Join-Path $installRoot 'xp2p.exe'
Write-Info "Copying xp2p.exe to $xp2pTarget"
Copy-Item $source $xp2pTarget -Force

Write-Info "Granting Modify rights to vagrant on $installRoot"
icacls $installRoot /grant 'vagrant:(OI)(CI)M' /t /c | Out-Null

Write-Info "Program Files xp2p preparation complete."
