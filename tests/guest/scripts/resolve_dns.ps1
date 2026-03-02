# Parameters:
# - Name: DNS name to resolve.
param(
    [Parameter(Mandatory = $true)]
    [string] $Name
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$target = $Name.Trim()
if (-not $target) {
    Write-Error 'Name is required.'
    exit 2
}

$resolved = Resolve-DnsName -Name $target -Type A -ErrorAction SilentlyContinue |
    Select-Object -First 1 -ExpandProperty IPAddress
if (-not $resolved) {
    Write-Error "Failed to resolve $target."
    exit 3
}

Write-Output $resolved
exit 0
