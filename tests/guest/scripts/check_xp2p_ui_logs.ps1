param(
    [Parameter(Mandatory = $true)]
    [string] $MarkerPath,
    [string] $Xp2pUiPath = "",
    [string] $LogPath = "C:\\ProgramData\\xp2p\\logs\\xp2p-ui.log",
    [int] $WaitSeconds = 8,
    [int] $MaxLines = 200
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

function Ensure-Marker([string] $path, [string] $payload) {
    if (-not $path) {
        return
    }
    $dir = Split-Path -Parent $path
    if ($dir) {
        [System.IO.Directory]::CreateDirectory($dir) | Out-Null
    }
    Set-Content -Path $path -Value $payload -Encoding ASCII
}

$exitCode = 0
$errorDetail = ""
$uiProc = $null

try {
    if (-not $Xp2pUiPath -or -not (Test-Path $Xp2pUiPath)) {
        $exitCode = 3
        $errorDetail = "xp2p-ui not found at $Xp2pUiPath"
        return
    }

    $uiProc = Start-Process -FilePath $Xp2pUiPath -PassThru
    if ($WaitSeconds -gt 0) {
        Start-Sleep -Seconds $WaitSeconds
    }

    if ($uiProc.HasExited) {
        $exitCode = 4
        $errorDetail = "xp2p-ui exited early"
        return
    }

    $deadline = [DateTime]::UtcNow.AddSeconds(10)
    while ([DateTime]::UtcNow -lt $deadline) {
        if (Test-Path $LogPath) {
            break
        }
        Start-Sleep -Seconds 1
    }

    if (-not (Test-Path $LogPath)) {
        $exitCode = 5
        $errorDetail = "log not found at $LogPath"
        return
    }

    $tail = Get-Content -Path $LogPath -ErrorAction SilentlyContinue -Tail $MaxLines
    $text = ($tail | Out-String).Trim()
    if ($text -match "access is denied") {
        $exitCode = 6
        $errorDetail = "log contains access denied"
        return
    }
    if ($text -match "xp2p-ui list services failed") {
        $exitCode = 7
        $errorDetail = "log contains list services failed"
        return
    }
} catch {
    $exitCode = 10
    $errorDetail = ($_ | Out-String).Trim()
} finally {
    if ($uiProc -and -not $uiProc.HasExited) {
        Stop-Process -Id $uiProc.Id -Force -ErrorAction SilentlyContinue
    }
    if ($exitCode -eq 0) {
        $payload = "OK"
    } else {
        $detail = $errorDetail -replace "[\r\n]+", " "
        if ($detail.Length -gt 160) {
            $detail = $detail.Substring(0, 160)
        }
        if ($detail) {
            $payload = "FAIL:${exitCode}:$detail"
        } else {
            $payload = "FAIL:${exitCode}"
        }
    }
    Ensure-Marker $MarkerPath $payload
}

exit $exitCode
