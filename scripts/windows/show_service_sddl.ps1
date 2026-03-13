param(
    [string[]] $ServiceName = @('xp2p-client', 'xp2p-server')
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

foreach ($name in $ServiceName) {
    $output = & sc.exe sdshow $name 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "sc.exe sdshow failed for $name: $output"
    }

    $sddlLine = $output | Where-Object { $_ -match '^(O:|D:|S:)' } | Select-Object -First 1
    if (-not $sddlLine) {
        $sddlLine = $output | Select-Object -Last 1
    }

    Write-Output $name
    Write-Output $sddlLine
    Write-Output ""
}
