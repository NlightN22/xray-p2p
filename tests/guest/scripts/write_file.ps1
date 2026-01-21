param(
    [Parameter(Mandatory = $true)]
    [string] $Path,

    [Parameter(Mandatory = $true)]
    [string] $ContentBase64
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$dir = Split-Path -Parent $Path
if ($dir -and -not (Test-Path $dir)) {
    New-Item -ItemType Directory -Path $dir -Force | Out-Null
}

$data = [System.Convert]::FromBase64String($ContentBase64)
[System.IO.File]::WriteAllBytes($Path, $data)
exit 0
