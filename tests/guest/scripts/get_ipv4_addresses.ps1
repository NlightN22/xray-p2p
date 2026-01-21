param()

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$addresses = Get-NetIPAddress -AddressFamily IPv4 -PrefixOrigin (@('Dhcp', 'Manual')) `
    | Where-Object { $_.IPAddress -ne '127.0.0.1' } `
    | Select-Object -ExpandProperty IPAddress
if (-not $addresses) {
    exit 3
}
$addresses
exit 0
