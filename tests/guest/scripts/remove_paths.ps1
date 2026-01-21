param(
    [Parameter(Mandatory = $true)]
    [string] $PathsBase64
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$decoded = [System.Text.Encoding]::UTF8.GetString(
    [System.Convert]::FromBase64String($PathsBase64)
)
$paths = @()
if ($decoded) {
    try {
        $paths = ConvertFrom-Json -InputObject $decoded -ErrorAction Stop
    } catch {
        $paths = @()
    }
}

if ($paths -is [string]) {
    $paths = @($paths)
} elseif (-not ($paths -is [System.Collections.IEnumerable])) {
    $paths = @()
}

foreach ($target in $paths) {
    if ($target -and (Test-Path $target)) {
        Remove-Item -Path $target -Force -Recurse -ErrorAction SilentlyContinue
    }
}

exit 0
