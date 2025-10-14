[CmdletBinding()]
param(
    [string]$ServerAddr = '10.0.0.1',
    [string]$UserName   = 'client1',
    [string]$ServerLan  = '10.0.101.0/24',
    [string]$ClientLan  = '10.0.102.0/24',
    [switch]$VerboseLogs
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$libPath = [System.IO.Path]::Combine($PSScriptRoot, '..', 'lib.ps1')
. $libPath

Set-VerboseLogs -Enabled:$VerboseLogs.IsPresent
Reset-TestResults

$result = Invoke-StageR2 -ServerAddr $ServerAddr -UserName $UserName -ServerLan $ServerLan -ClientLan $ClientLan
$results = Publish-TestResults
$result
if (($results | Where-Object { $_.Status -eq 'FAIL' }).Count -gt 0) {
    exit 1
}
