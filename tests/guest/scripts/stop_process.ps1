param(
    [Parameter(Mandatory = $true)]
    [int] $ProcessId
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

if ($ProcessId -le 0) {
    exit 0
}

$proc = Get-Process -Id $ProcessId -ErrorAction SilentlyContinue
if ($proc) {
    Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
}
exit 0
