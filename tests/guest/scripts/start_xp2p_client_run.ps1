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

    [int] $StabilizeSeconds = 6,

    [string] $AllowMismatch = "",

    [string] $OutputLogPath = ""
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

if (-not (Test-Path $Xp2pPath)) {
    Write-Output '__XP2P_MISSING__'
    exit 3
}

$services = @('xp2p-client', 'xp2p-server')
foreach ($name in $services) {
    $svc = Get-Service -Name $name -ErrorAction SilentlyContinue
    if ($svc -and $svc.Status -ne 'Stopped') {
        Stop-Service -Name $name -Force -ErrorAction SilentlyContinue
    }
}
function Stop-ListeningPorts {
    param([int[]]$Ports)
    $targets = @{}
    foreach ($port in $Ports) {
        $targets[$port] = $true
    }
    $netCmd = Get-Command Get-NetTCPConnection -ErrorAction SilentlyContinue
    if ($netCmd) {
        foreach ($port in $Ports) {
            $listeners = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue
            foreach ($listener in $listeners) {
                if ($listener.OwningProcess -gt 0) {
                    try {
                        Stop-Process -Id $listener.OwningProcess -Force -ErrorAction SilentlyContinue
                    } catch { }
                }
            }
        }
        return
    }
    $lines = netstat -ano -p tcp | Select-String -Pattern "LISTENING"
    foreach ($match in $lines) {
        $line = $match.Line
        if (-not $line) {
            continue
        }
        $parts = $line -split "\s+"
        if ($parts.Length -lt 5) {
            continue
        }
        $local = $parts[1]
        $pid = $parts[-1]
        if ($local -match ":(\\d+)$") {
            $port = [int]$Matches[1]
            if ($targets.ContainsKey($port)) {
                try {
                    Stop-Process -Id $pid -Force -ErrorAction SilentlyContinue
                } catch { }
            }
        }
    }
}
Stop-ListeningPorts -Ports @(51080, 51180)
Start-Sleep -Seconds 1

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

$commandLine = "`"$Xp2pPath`" client run --quiet --auto-install --path `"$InstallDir`" --config-dir `"$ConfigDir`" --xray-log-file `"$LogRelative`""
$envPrefix = ""
if ($AllowMismatch -and $AllowMismatch -ne "0") {
    $envPrefix = "set XP2P_XRAY_ALLOW_MISMATCH=1&& "
}

$redirect = ""
if ($OutputLogPath) {
    $outputDir = Split-Path -Parent $OutputLogPath
    if ($outputDir -and -not (Test-Path $outputDir)) {
        New-Item -ItemType Directory -Path $outputDir -Force | Out-Null
    }
    if (Test-Path $OutputLogPath) {
        Remove-Item $OutputLogPath -Force -ErrorAction SilentlyContinue
    }
    New-Item -ItemType File -Path $OutputLogPath -Force | Out-Null
    $redirect = " > `"$OutputLogPath`" 2>&1"
}

if ($envPrefix -or $redirect) {
    $wrapped = $envPrefix + $commandLine + $redirect
    $commandLine = "cmd.exe /c `"$wrapped`""
}
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
