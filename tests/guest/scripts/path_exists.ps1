param(
    [string] $TargetPath,
    [string] $Path
)

$ErrorActionPreference = 'Stop'

if (-not $TargetPath) {
    $TargetPath = $Path
}
if (-not $TargetPath -and $args.Count -gt 0) {
    $TargetPath = $args[0]
}
if (-not $TargetPath) {
    Write-Error "Path is required"
    exit 1
}

try {
    if (Test-Path -LiteralPath $TargetPath) {
        Write-Output "EXISTS"
        exit 0
    }
    exit 3
} catch {
    Write-Error $_
    exit 1
}

exit 3
