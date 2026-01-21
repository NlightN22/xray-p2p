param(
    [Parameter(Mandatory = $true)]
    [string] $Path
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

if (Test-Path $Path) {
    Write-Output "EXISTS"
    exit 0
}

exit 3
