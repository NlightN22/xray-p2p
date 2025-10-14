[CmdletBinding()]
param(
    [string]$ServerAddr = '10.0.0.1',
    [string]$UserName   = 'client3',
    [string]$ServerLan  = '10.0.101.0/24',
    [string]$ClientLan  = '10.0.103.0/24',
    [string]$C1Ip,
    [switch]$VerboseLogs
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$libPath = [System.IO.Path]::Combine($PSScriptRoot, '..', 'lib.ps1')
. $libPath

Set-VerboseLogs -Enabled:$VerboseLogs.IsPresent
Reset-TestResults

$result = Invoke-StageR3 -ServerAddr $ServerAddr -UserName $UserName -ServerLan $ServerLan -ClientLan $ClientLan -C1Ip $C1Ip
$results = Publish-TestResults
$result
if (($results | Where-Object { $_.Status -eq 'FAIL' }).Count -gt 0) {
    exit 1
}
