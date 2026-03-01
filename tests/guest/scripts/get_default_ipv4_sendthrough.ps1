# Parameters: none.
$ErrorActionPreference = "Stop"

$def = Get-NetRoute -DestinationPrefix "0.0.0.0/0" -AddressFamily IPv4 |
    Where-Object { $_.NextHop -ne "0.0.0.0" } |
    Sort-Object RouteMetric,ifMetric |
    Select-Object -First 1
if ($null -eq $def) {
    exit 0
}

$ip = Get-NetIPAddress -AddressFamily IPv4 -InterfaceIndex $def.ifIndex |
    Where-Object { $_.IPAddress -notlike "169.254.*" -and $_.IPAddress -ne "127.0.0.1" -and $_.IPAddress -ne "0.0.0.0" } |
    Select-Object -First 1 -ExpandProperty IPAddress
if ($null -ne $ip) {
    Write-Output $ip
}
