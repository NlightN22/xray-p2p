# Parameters:
# - InterfaceName: adapter name to query.
param(
    [Parameter(Mandatory = $true)]
    [string] $InterfaceName
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$name = $InterfaceName.Trim()
if (-not $name) {
    Write-Error 'InterfaceName is required.'
    exit 2
}

$adapters = $null
try {
    $adapters = Get-NetAdapter -IncludeHidden -ErrorAction Stop
} catch {
    $adapters = Get-NetAdapter -ErrorAction SilentlyContinue
}

$adapter = $adapters | Where-Object { $_.Name -eq $name } | Select-Object -First 1
if (-not $adapter) {
    $adapter = $adapters |
        Where-Object { $_.Name -like "$name*" } |
        Sort-Object @{ Expression = { if ($_.Status -eq 'Up') { 1 } else { 0 } }; Descending = $true }, `
            @{ Expression = { $_.ifIndex }; Descending = $true } |
        Select-Object -First 1
}
if (-not $adapter) {
    $adapter = $adapters |
        Where-Object { $_.InterfaceDescription -like '*Wintun*' -or $_.InterfaceDescription -like '*Xray Tunnel*' -or $_.Name -like '*Xray Tunnel*' } |
        Sort-Object @{ Expression = { if ($_.Status -eq 'Up') { 1 } else { 0 } }; Descending = $true }, `
            @{ Expression = { $_.ifIndex }; Descending = $true } |
        Select-Object -First 1
}
if (-not $adapter) {
    Write-Error "Adapter not found: $name"
    exit 3
}

Write-Output $adapter.ifIndex
exit 0
