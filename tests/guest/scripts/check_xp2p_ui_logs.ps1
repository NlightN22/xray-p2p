param(
    [Parameter(Mandatory = $true)]
    [string] $MarkerPath,
    [string] $Xp2pUiPath = "",
    [string] $LogPath = "C:\\ProgramData\\xp2p\\logs\\xp2p-ui.log",
    [int] $WaitSeconds = 8,
    [int] $MaxLines = 200,
    [string] $ClearLog = "true"
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

function Get-ServiceSddl([string] $name) {
    $output = & sc.exe sdshow $name 2>&1
    if ($LASTEXITCODE -ne 0) {
        return $null
    }
    $joined = ($output -join "`n").Trim()
    $match = [regex]::Match($joined, "O:.*")
    if (-not $match.Success) {
        $match = [regex]::Match($joined, "D:.*")
    }
    if (-not $match.Success) {
        return $null
    }
    return $match.Value.Trim()
}

$uiPid = $null

$exitCode = 0
$errorDetail = ""
$aclSnapshot = ""
$uiProc = $null

try {
    if (-not $Xp2pUiPath -or -not (Test-Path $Xp2pUiPath)) {
        $exitCode = 3
        $errorDetail = "xp2p-ui not found at $Xp2pUiPath"
        return
    }

    if ($ClearLog -and $ClearLog.ToLowerInvariant() -ne "false") {
        $logBackupDir = "C:\\xp2p\\build\\ui-logs"
        New-Item -ItemType Directory -Path $logBackupDir -Force | Out-Null
        $aclLines = @()
        foreach ($svc in @("xp2p-client", "xp2p-server")) {
            $sddl = Get-ServiceSddl $svc
            if ($sddl) {
                $aclLines += "$svc $sddl"
            } else {
                $aclLines += "$svc <no-sddl>"
            }
        }
        if ($aclLines.Count -gt 0) {
            $aclSnapshot = ($aclLines -join " | ")
            $aclPath = Join-Path $logBackupDir ("service-sddl-{0}.txt" -f (Get-Date -Format "yyyyMMddHHmmss"))
            Set-Content -Path $aclPath -Value $aclLines -Encoding ASCII
        }
        if (Test-Path $LogPath) {
            $backup = Join-Path $logBackupDir ("xp2p-ui-{0}.log" -f (Get-Date -Format "yyyyMMddHHmmss"))
            Copy-Item -Path $LogPath -Destination $backup -Force
            Remove-Item -Path $LogPath -Force -ErrorAction SilentlyContinue
        }
    }

    $uiProc = Start-Process -FilePath $Xp2pUiPath -PassThru
    if (-not $uiProc) {
        $exitCode = 4
        $errorDetail = "xp2p-ui process failed to start"
        return
    }
    $uiPid = $uiProc.Id
    if ($WaitSeconds -gt 0) {
        Start-Sleep -Seconds $WaitSeconds
    }
    $running = Get-Process -Id $uiPid -ErrorAction SilentlyContinue
    if (-not $running) {
        $exitCode = 4
        $errorDetail = "xp2p-ui exited before log check"
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
$tailText = $text -replace '[^\x00-\x7F]', '?'
$tailText = $tailText -replace "(`r`n|`r|`n)+", "`n"
Write-Host "xp2p-ui log tail ($MaxLines lines max):"
if ($tailText) {
    Write-Host $tailText
} else {
    Write-Host "<empty>"
}
    if ($text -match "access is denied") {
        $exitCode = 6
        $errorDetail = "log contains access denied"
        if ($aclSnapshot) {
            $errorDetail = "$errorDetail; sddl=$aclSnapshot"
        }
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
    if ($uiPid) {
        Stop-Process -Id $uiPid -Force -ErrorAction SilentlyContinue
    }
    if ($exitCode -eq 0) {
        $payload = "OK"
        if ($tailText) {
            $payload = $payload + "`n" + $tailText
        }
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
