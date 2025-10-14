[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)][string]$C1Ip,
    [Parameter(Mandatory=$true)][string]$C2Ip,
    [Parameter(Mandatory=$true)][string]$C3Ip,
    [switch]$VerboseLogs
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$libPath = [System.IO.Path]::Combine($PSScriptRoot, '..', 'lib.ps1')
. $libPath

Set-VerboseLogs -Enabled:$VerboseLogs.IsPresent
Reset-TestResults

Test-ReverseTunnels -C1Ip $C1Ip -C2Ip $C2Ip -C3Ip $C3Ip
$results = Publish-TestResults
if (($results | Where-Object { $_.Status -eq 'FAIL' }).Count -gt 0) {
    exit 1
}
