param(
    [Parameter(Mandatory = $true)]
    [int] $ProcessId
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

if ($ProcessId -le 0) {
    exit 3
}

$proc = Get-Process -Id $ProcessId -ErrorAction SilentlyContinue
if ($proc) {
    exit 0
}
exit 3
