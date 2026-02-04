param()

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$projectRoot = Resolve-Path (Join-Path $scriptDir '..\..')
$pinFile = Join-Path $projectRoot 'distro\xray\pinned.json'

if (-not (Test-Path $pinFile)) {
    throw "Pinned file not found at $pinFile"
}

$data = Get-Content -Raw -Path $pinFile | ConvertFrom-Json
$targets = $data.targets
$errors = New-Object System.Collections.Generic.List[string]

foreach ($target in ($targets.PSObject.Properties | Sort-Object Name)) {
    $targetName = $target.Name
    $meta = $target.Value
    if (-not $targetName.Contains('/')) {
        $errors.Add("$targetName: expected target format os/arch")
        continue
    }
    $parts = $targetName.Split('/', 2)
    $osName = $parts[0]
    $arch = $parts[1]
    foreach ($item in $meta.files) {
        $name = $item.name
        $expected = $item.sha256
        $required = [bool]$item.required
        if ([string]::IsNullOrWhiteSpace($name) -or [string]::IsNullOrWhiteSpace($expected)) {
            $errors.Add("$targetName: invalid file entry")
            continue
        }
        $path = Join-Path $projectRoot ("distro\{0}\bundle\{1}\{2}" -f $osName, $arch, $name)
        if (-not (Test-Path $path)) {
            if ($required) {
                $errors.Add("$targetName: missing $path")
            }
            continue
        }
        $actual = (Get-FileHash -Algorithm SHA256 -Path $path).Hash.ToLower()
        if ($actual -ne $expected.ToLower()) {
            $errors.Add("$targetName: sha256 mismatch for $path: expected $expected, got $actual")
        }
    }
}

if ($errors.Count -gt 0) {
    foreach ($err in $errors) {
        Write-Error $err
    }
    exit 1
}

Write-Host "Pinned xray assets OK"
