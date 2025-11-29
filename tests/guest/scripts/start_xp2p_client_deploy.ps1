param(
    [Parameter(Mandatory = $true)]
    [string] $Xp2pPath,

    [Parameter(Mandatory = $true)]
    [string] $LogPath,

    [Parameter(Mandatory = $true)]
    [string] $RemoteHost,

    [Parameter(Mandatory = $true)]
    [string] $DeployPort,

    [Parameter(Mandatory = $true)]
    [string] $TrojanUser,

    [Parameter(Mandatory = $true)]
    [string] $TrojanPassword,

    [string] $TrojanPort,

    [string[]] $AdditionalArgs
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

if (-not (Test-Path $Xp2pPath)) {
    Write-Output '__XP2P_MISSING__'
    exit 3
}

$logDir = Split-Path $LogPath -Parent
if ($logDir -and -not (Test-Path $logDir)) {
    New-Item -ItemType Directory -Path $logDir -Force | Out-Null
}

$stderrPath = "$LogPath.err"
$runtimeRoot = Join-Path ([System.IO.Path]::GetTempPath()) 'xp2p-deploy'
if (-not (Test-Path $runtimeRoot)) {
    New-Item -ItemType Directory -Path $runtimeRoot -Force | Out-Null
}
$taskId = [guid]::NewGuid().ToString()
$taskName = "xp2p-client-deploy-$taskId"
$launcherPath = Join-Path $runtimeRoot "$taskId.ps1"
$pidPath = Join-Path $runtimeRoot "$taskId.pid"
foreach ($target in @($LogPath, $stderrPath)) {
    if (Test-Path $target) {
        Remove-Item $target -Force -ErrorAction SilentlyContinue
    }
}

$arguments = @(
    'client', 'deploy',
    '--host', $RemoteHost,
    '--port', $DeployPort,
    '--user', $TrojanUser,
    '--password', $TrojanPassword
)

if ($TrojanPort) {
    $arguments += @('--trojan-port', $TrojanPort)
}
if ($AdditionalArgs) {
    $arguments += $AdditionalArgs
}

$escapedArgs = $arguments | ForEach-Object { "'" + ($_ -replace "'", "''") + "'" }
$argListLiteral = [string]::Join(',', $escapedArgs)
$launcher = @"
$ErrorActionPreference = 'Stop'
`$argsList = @($argListLiteral)
`$proc = Start-Process -FilePath '$Xp2pPath' -ArgumentList `$argsList -RedirectStandardOutput '$LogPath' -RedirectStandardError '$stderrPath' -WindowStyle Hidden -PassThru
Set-Content -Path '$pidPath' -Value `$proc.Id -Encoding ASCII
"@

Set-Content -Path $launcherPath -Value $launcher -Encoding UTF8

try {
    $action = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument "-NoProfile -ExecutionPolicy Bypass -File `"$launcherPath`""
    $trigger = New-ScheduledTaskTrigger -Once -At ((Get-Date).AddMinutes(5))
    Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger -RunLevel Highest -User 'SYSTEM' | Out-Null
    Start-ScheduledTask -TaskName $taskName

    $deadline = (Get-Date).AddSeconds(20)
    while (-not (Test-Path $pidPath)) {
        Start-Sleep -Milliseconds 200
        if ((Get-Date) -gt $deadline) {
            throw "xp2p client deploy task did not record PID"
        }
    }
}
finally {
    Unregister-ScheduledTask -TaskName $taskName -Confirm:$false -ErrorAction SilentlyContinue | Out-Null
    Remove-Item $launcherPath -Force -ErrorAction SilentlyContinue
}

try {
    $procId = Get-Content -Path $pidPath -Raw
} catch {
    $procId = ''
}
Remove-Item $pidPath -Force -ErrorAction SilentlyContinue

if (-not $procId) {
    Write-Output '__XP2P_EXIT__'
    exit 5
}

Write-Output ('PID=' + $procId)
Write-Output ('STDOUT=' + $LogPath)
Write-Output ('STDERR=' + $stderrPath)
exit 0
