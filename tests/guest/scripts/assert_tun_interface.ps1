# Parameters:
# - InterfaceName: TUN adapter name to validate.
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

$adapter = Get-NetAdapter -Name $name -ErrorAction SilentlyContinue | Select-Object -First 1
if (-not $adapter) {
    Write-Error "TUN adapter not found: $name"
    exit 3
}

exit 0
