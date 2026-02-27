param(
    [string]$DnsName = "example.com",
    [string]$TcpHost = "1.1.1.1",
    [int]$TcpPort = 443
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

try {
    Resolve-DnsName -Name $DnsName -ErrorAction Stop | Out-Null
} catch {
    Write-Error "Internet check failed: DNS lookup for $DnsName"
    exit 1
}

try {
    $tcpOk = Test-NetConnection -ComputerName $TcpHost -Port $TcpPort -InformationLevel Quiet
} catch {
    $tcpOk = $false
}
if (-not $tcpOk) {
    Write-Error "Internet check failed: TCP connect to $TcpHost:$TcpPort"
    exit 1
}

exit 0
