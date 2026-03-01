param(
    [string] $HintPath
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$candidates = @()
if ($HintPath) {
    $candidates += $HintPath
}
$candidates += @(
    'C:\Program Files\xp2p\xp2p.exe',
    'C:\Program Files\xp2p\bin\xp2p.exe',
    'C:\Program Files (x86)\xp2p\xp2p.exe',
    'C:\Program Files (x86)\xp2p\bin\xp2p.exe',
    'C:\ProgramData\xp2p\xp2p.exe',
    'C:\ProgramData\xp2p\bin\xp2p.exe'
)

foreach ($candidate in $candidates) {
    if ($candidate -and (Test-Path $candidate)) {
        Write-Output $candidate
        exit 0
    }
}

$roots = @(
    'C:\Program Files',
    'C:\Program Files (x86)',
    'C:\ProgramData'
)

foreach ($root in $roots) {
    if (-not (Test-Path $root)) {
        continue
    }
    $found = Get-ChildItem -Path $root -Filter xp2p.exe -Recurse -ErrorAction SilentlyContinue |
        Select-Object -First 1 -ExpandProperty FullName
    if ($found) {
        Write-Output $found
        exit 0
    }
}

$usersRoot = 'C:\Users'
if (Test-Path $usersRoot) {
    $users = Get-ChildItem -Path $usersRoot -Directory -ErrorAction SilentlyContinue
    foreach ($user in $users) {
        $root = Join-Path $user.FullName 'AppData\Local\Programs'
        if (-not (Test-Path $root)) {
            continue
        }
        $found = Get-ChildItem -Path $root -Filter xp2p.exe -Recurse -ErrorAction SilentlyContinue |
            Select-Object -First 1 -ExpandProperty FullName
        if ($found) {
            Write-Output $found
            exit 0
        }
    }
}

exit 3
