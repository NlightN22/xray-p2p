param(
    [Parameter(Mandatory = $true)]
    [int] $Pid
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

if ($Pid -le 0) {
    exit 0
}

$proc = Get-Process -Id $Pid -ErrorAction SilentlyContinue
if ($proc) {
    Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
}
exit 0
