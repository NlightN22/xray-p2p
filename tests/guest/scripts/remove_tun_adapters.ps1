param(
    [Parameter(Mandatory = $true)]
    [string] $NamesBase64
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$payload = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($NamesBase64))
$names = $payload | ConvertFrom-Json

$removeCmd = Get-Command Remove-NetAdapter -ErrorAction SilentlyContinue
$disableCmd = Get-Command Disable-NetAdapter -ErrorAction SilentlyContinue

foreach ($name in $names) {
    if (-not $name) {
        continue
    }
    $adapter = Get-NetAdapter -Name $name -ErrorAction SilentlyContinue | Select-Object -First 1
    if (-not $adapter) {
        continue
    }
    if ($removeCmd) {
        Remove-NetAdapter -Name $adapter.Name -Confirm:$false -ErrorAction SilentlyContinue
        continue
    }
    if ($disableCmd) {
        Disable-NetAdapter -Name $adapter.Name -Confirm:$false -ErrorAction SilentlyContinue
    }
}
exit 0
