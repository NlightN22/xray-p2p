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

$process = Start-Process -FilePath $Xp2pPath -ArgumentList $arguments -RedirectStandardOutput $LogPath -RedirectStandardError $stderrPath -WindowStyle Hidden -PassThru
Start-Sleep -Seconds 1
if ($process.HasExited) {
    Write-Output '__XP2P_EXIT__'
    exit 5
}

Write-Output ('PID=' + $process.Id)
Write-Output ('STDOUT=' + $LogPath)
Write-Output ('STDERR=' + $stderrPath)
exit 0
