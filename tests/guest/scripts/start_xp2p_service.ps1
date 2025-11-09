param(
    [Parameter(Mandatory = $true)]
    [string] $Xp2pPath,

    [Parameter(Mandatory = $true)]
    [int] $Port,

    [int] $TimeoutSeconds = 60
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
    $remaining = Get-Process -Name xp2p -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $Xp2pPath }
    if ($remaining) {
        Write-Output '__XP2P_ALREADY_RUNNING__'
        exit 7
    }
}

$escapedPath = $Xp2pPath.Replace("'", "''")
$commandLine = "powershell -NoProfile -Command `"& { `$env:XP2P_SERVER_PORT = '$Port'; & '$escapedPath' }`""
$workingDir = Split-Path $Xp2pPath
$createResult = Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{ CommandLine = $commandLine; CurrentDirectory = $workingDir }
if ($createResult.ReturnValue -ne 0 -or -not $createResult.ProcessId) {
    Write-Output ('__XP2P_CREATE_FAIL__' + $createResult.ReturnValue)
    exit 4
}
$processId = [int]$createResult.ProcessId
$deadline = (Get-Date).AddSeconds($TimeoutSeconds)

while ((Get-Date) -lt $deadline) {
    $proc = Get-Process -Id $processId -ErrorAction SilentlyContinue
    if (-not $proc) {
        Write-Output '__XP2P_EXIT__'
        exit 6
    }
    if (Test-NetConnection -ComputerName '127.0.0.1' -Port $Port -InformationLevel Quiet) {
        Write-Output ('PID=' + $processId)
        exit 0
    }
    Start-Sleep -Seconds 1
}

$proc = Get-Process -Id $processId -ErrorAction SilentlyContinue
if ($proc) {
    try {
        Stop-Process -Id $processId -Force -ErrorAction SilentlyContinue
    } catch { }
}
Write-Output '__XP2P_TIMEOUT__'
exit 5
