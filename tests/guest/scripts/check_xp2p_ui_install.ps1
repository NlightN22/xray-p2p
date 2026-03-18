param(
    [Parameter(Mandatory = $true)]
    [string] $Xp2pUiPath,
    [Parameter(Mandatory = $true)]
    [string] $MarkerPath,
    [string] $AutostartExpected = "true",
    [string] $RunValueName = "ui-xp2p"
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$exitCode = 0
try {
    if (-not (Test-Path $Xp2pUiPath)) {
        Write-Output "ui-xp2p not found at $Xp2pUiPath"
        $exitCode = 3
        return
    }

    $expect = $AutostartExpected.Trim().ToLowerInvariant()
    if ($expect -and $expect -ne "skip" -and $expect -ne "auto") {
        $runKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
        $present = $false
        if (Test-Path $runKey) {
            $value = Get-ItemProperty -Path $runKey -Name $RunValueName -ErrorAction SilentlyContinue
            if ($null -ne $value) {
                $present = $true
            }
        }
        if ($expect -in @("1", "true", "yes")) {
            if (-not $present) {
                Write-Output "ui-xp2p autostart missing in HKCU Run"
                $exitCode = 4
                return
            }
        } elseif ($expect -in @("0", "false", "no")) {
            if ($present) {
                Write-Output "ui-xp2p autostart present in HKCU Run when disabled"
                $exitCode = 5
                return
            }
        }
    }
} catch {
    Write-Output "check_xp2p_ui_install failed: $($_.Exception.Message)"
    $exitCode = 10
} finally {
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
