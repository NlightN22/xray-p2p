[CmdletBinding()]
param(
    [int]$MaxAttempts = 2,
    [switch]$NoDestroyBetweenAttempts,
    [switch]$VerboseLogs
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$libPath = [System.IO.Path]::Combine($PSScriptRoot, '..', 'lib.ps1')
. $libPath

Set-VerboseLogs -Enabled:$VerboseLogs.IsPresent
Reset-TestResults

$destroy = -not $NoDestroyBetweenAttempts
Start-VagrantEnvironment -MaxAttempts $MaxAttempts -DestroyBetweenAttempts:$destroy
$results = @(Publish-TestResults)
$hasFail = $false
foreach ($res in $results) {
    if ($res.Status -eq 'FAIL') {
        $hasFail = $true
        break
    }
}
if ($hasFail) {
    exit 1
}
