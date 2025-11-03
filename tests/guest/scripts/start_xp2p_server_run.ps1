param(
    [Parameter(Mandatory = $true)]
    [string] $Xp2pPath,

    [Parameter(Mandatory = $true)]
    [string] $InstallDir,

    [Parameter(Mandatory = $true)]
    [string] $ConfigDir,

    [Parameter(Mandatory = $true)]
    [string] $LogRelative,

    [Parameter(Mandatory = $true)]
    [string] $LogPath,

    [int] $StabilizeSeconds = 6
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

if (-not (Test-Path $Xp2pPath)) {
    Write-Output '__XP2P_MISSING__'
    exit 3
}

$existing = Get-Process -Name xp2p -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $Xp2pPath }
if ($existing) {
    foreach ($item in $existing) {
        try {
            Stop-Process -Id $item.Id -Force -ErrorAction SilentlyContinue
        } catch { }
    }
    Start-Sleep -Seconds 1
}

$xrayExisting = Get-Process -Name xray -ErrorAction SilentlyContinue
if ($xrayExisting) {
    foreach ($item in $xrayExisting) {
        try {
            Stop-Process -Id $item.Id -Force -ErrorAction SilentlyContinue
        } catch { }
    }
    Start-Sleep -Seconds 1
}

if (Test-Path $LogPath) {
    Remove-Item $LogPath -Force -ErrorAction SilentlyContinue
}

$commandLine = "`"$Xp2pPath`" server run --quiet --path `"$InstallDir`" --config-dir `"$ConfigDir`" --xray-log-file `"$LogRelative`""
$workingDir = Split-Path $Xp2pPath
$createResult = Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{ CommandLine = $commandLine; CurrentDirectory = $workingDir }
if ($createResult.ReturnValue -ne 0 -or -not $createResult.ProcessId) {
    Write-Output ('__XP2P_CREATE_FAIL__' + $createResult.ReturnValue)
    exit 4
}
$processId = [int]$createResult.ProcessId
$deadline = (Get-Date).AddSeconds($StabilizeSeconds)

while ((Get-Date) -lt $deadline) {
    $proc = Get-Process -Id $processId -ErrorAction SilentlyContinue
    if (-not $proc) {
        Write-Output '__XP2P_EXIT__'
        exit 6
    }
    $xray = Get-Process -Name xray -ErrorAction SilentlyContinue
    if ($xray) {
        Write-Output ('PID=' + $processId)
        exit 0
    }
    Start-Sleep -Seconds 1
}

Write-Output '__XP2P_TIMEOUT__'
exit 5
