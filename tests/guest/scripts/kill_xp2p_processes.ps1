param()

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

foreach ($name in @('xp2p', 'xray')) {
    $procs = Get-Process -Name $name -ErrorAction SilentlyContinue
    if (-not $procs) {
        continue
    }
    foreach ($proc in $procs) {
        try {
            Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
        } catch { }
    }
}
