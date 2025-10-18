[CmdletBinding()]
param(
    [string]$ServerAddr = '10.0.0.1',
    [string]$UserName   = 'client1',
    [string]$ServerLan  = '10.0.101.0/24',
    [string]$ClientLan  = '10.0.102.0/24',
    [switch]$VerboseLogs,
    [string]$RepoBaseUrl,
    [string]$SshUser,
    [int]$SshPort,
    [int]$ServerPort,
    [string]$CertFile,
    [string]$KeyFile
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$libPath = [System.IO.Path]::Combine($PSScriptRoot, '..', 'lib.ps1')
. $libPath

Set-VerboseLogs -Enabled:$VerboseLogs.IsPresent
Reset-TestResults

$result = Invoke-StageR2 -ServerAddr $ServerAddr -UserName $UserName -ServerLan $ServerLan -ClientLan $ClientLan -RepoBaseUrl $RepoBaseUrl -SshUser $SshUser -SshPort $SshPort -ServerPort $ServerPort -CertFile $CertFile -KeyFile $KeyFile
$results = @(Publish-TestResults)
$result
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
