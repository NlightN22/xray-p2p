param(
    [Parameter(Mandatory = $true)]
    [string] $Xp2pUiPath,
    [Parameter(Mandatory = $true)]
    [string] $MarkerPath,
    [int] $WaitSeconds = 8
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$exitCode = 0
$proc = $null
try {
    if (-not (Test-Path $Xp2pUiPath)) {
        Write-Output "xp2p-ui not found at $Xp2pUiPath"
        $exitCode = 3
        return
    }
    $proc = Start-Process -FilePath $Xp2pUiPath -PassThru
    if (-not $proc) {
        Write-Output "xp2p-ui process failed to start"
        $exitCode = 4
        return
    }
    if ($WaitSeconds -gt 0) {
        Start-Sleep -Seconds $WaitSeconds
    }
    $running = Get-Process -Id $proc.Id -ErrorAction SilentlyContinue
    if (-not $running) {
        Write-Output "xp2p-ui exited before smoke timeout"
        $exitCode = 5
        return
    }
    Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
    try {
        Wait-Process -Id $proc.Id -Timeout 10 -ErrorAction SilentlyContinue
    } catch {
        $null = $null
    }
    $remaining = Get-Process -Id $proc.Id -ErrorAction SilentlyContinue
    if ($remaining) {
        Write-Output "xp2p-ui process still running after stop"
        $exitCode = 6
        return
    }
} catch {
    Write-Output "launch_xp2p_ui failed: $($_.Exception.Message)"
    $exitCode = 10
} finally {
    if ($proc -and -not $proc.HasExited) {
        Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
    }
    if ($MarkerPath) {
        $markerDir = Split-Path -Parent $MarkerPath
        if ($markerDir) {
            [System.IO.Directory]::CreateDirectory($markerDir) | Out-Null
        }
        $payload = if ($exitCode -eq 0) { "OK" } else { "FAIL:$exitCode" }
        Set-Content -Path $MarkerPath -Value $payload -Encoding ASCII
    }
}

exit $exitCode
