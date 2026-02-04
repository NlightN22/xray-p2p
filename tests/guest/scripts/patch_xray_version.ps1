param(
    [Parameter(Mandatory = $true)]
    [string] $XrayPath,

    [Parameter(Mandatory = $true)]
    [string] $BackupPath,

    [Parameter(Mandatory = $true)]
    [string] $ExpectedVersion,

    [Parameter(Mandatory = $true)]
    [string] $ReplacementVersion
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

if (-not (Test-Path $XrayPath)) {
    throw "xray binary not found at $XrayPath"
}

if ($ExpectedVersion.Length -ne $ReplacementVersion.Length) {
    throw "Expected and replacement versions must have equal length."
}

if (-not (Test-Path $BackupPath)) {
    Copy-Item -Path $XrayPath -Destination $BackupPath -Force
}

$data = [System.IO.File]::ReadAllBytes($XrayPath)
$expectedBytes = [System.Text.Encoding]::ASCII.GetBytes($ExpectedVersion)
$replacementBytes = [System.Text.Encoding]::ASCII.GetBytes($ReplacementVersion)

$matches = 0
for ($i = 0; $i -le $data.Length - $expectedBytes.Length; $i++) {
    $found = $true
    for ($j = 0; $j -lt $expectedBytes.Length; $j++) {
        if ($data[$i + $j] -ne $expectedBytes[$j]) {
            $found = $false
            break
        }
    }
    if ($found) {
        [System.Array]::Copy($replacementBytes, 0, $data, $i, $replacementBytes.Length)
        $matches++
        $i += $expectedBytes.Length - 1
    }
}

if ($matches -le 0) {
    throw "Version string '$ExpectedVersion' not found in $XrayPath"
}

[System.IO.File]::WriteAllBytes($XrayPath, $data)
Write-Output ("__XP2P_REPLACED__=" + $matches)
