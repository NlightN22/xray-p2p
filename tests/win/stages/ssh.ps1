[CmdletBinding()]
param(
    [string[]]$Machines = @('r1','r2','r3','c1','c2','c3'),
    [switch]$VerboseLogs
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$libPath = [System.IO.Path]::Combine($PSScriptRoot, '..', 'lib.ps1')
. $libPath

Set-VerboseLogs -Enabled:$VerboseLogs.IsPresent
Reset-TestResults

Test-SshReadiness -Machines $Machines
$results = Publish-TestResults
if (($results | Where-Object { $_.Status -eq 'FAIL' }).Count -gt 0) {
    exit 1
}
