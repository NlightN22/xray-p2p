# Parameters:
# - DestinationPrefix: CIDR to query.
# - InterfaceIndex: optional interface index to filter.
param(
    [Parameter(Mandatory = $true)]
    [string] $DestinationPrefix,
    [string] $InterfaceIndex
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$prefix = $DestinationPrefix.Trim()
if (-not $prefix) {
    Write-Error 'DestinationPrefix is required.'
    exit 2
}

$routes = $null
$indexValue = $InterfaceIndex
if ($null -eq $indexValue) {
    $indexValue = ''
}
$indexValue = $indexValue.Trim()
if ($indexValue) {
    $parsed = 0
    if (-not [int]::TryParse($indexValue, [ref]$parsed)) {
        Write-Error "InterfaceIndex must be an integer, got: $InterfaceIndex"
        exit 2
    }
    $routes = Get-NetRoute -DestinationPrefix $prefix -InterfaceIndex $parsed -ErrorAction SilentlyContinue
} else {
    $routes = Get-NetRoute -DestinationPrefix $prefix -ErrorAction SilentlyContinue
}

$items = @()
foreach ($route in @($routes)) {
    if (-not $route) {
        continue
    }
    $items += [pscustomobject]@{
        DestinationPrefix = $route.DestinationPrefix
        InterfaceIndex    = $route.InterfaceIndex
        InterfaceAlias    = $route.InterfaceAlias
        NextHop           = $route.NextHop
        RouteMetric       = $route.RouteMetric
    }
}

$items | ConvertTo-Json -Depth 3 -Compress
exit 0
