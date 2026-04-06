$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Write-Info {
    param(
        [Parameter(Mandatory = $true)]
        [string] $Message
    )

    Write-Host "==> $Message"
}

$target = "C:\Windows\Temp"
Write-Info "Granting Users modify access to $target"

if (-not (Test-Path -LiteralPath $target)) {
    throw "Target path not found: $target"
}

icacls $target /grant "Users:(OI)(CI)(M)" | Write-Host
Write-Info "Windows Temp ACL update completed."
