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

function Ensure-BatchLogonRight([string] $userPath) {
    $account = New-Object System.Security.Principal.NTAccount($userPath)
    $sid = $account.Translate([System.Security.Principal.SecurityIdentifier]).Value
    if (-not $sid) {
        throw "Unable to resolve SID for $userPath"
    }
    $sidEntry = "*$sid"

    $policyDir = "C:\\xp2p\\build\\ui-secpol"
    New-Item -ItemType Directory -Path $policyDir -Force | Out-Null
    $cfgPath = Join-Path $policyDir ("secpol-{0}-{1}.cfg" -f (Get-Date -Format "yyyyMMddHHmmss"), ([System.Guid]::NewGuid().ToString("N").Substring(0, 6)))
    $dbPath = Join-Path $policyDir "secpol.sdb"

    $exportOutput = & secedit.exe /export /cfg $cfgPath /areas USER_RIGHTS 2>&1
    if ($LASTEXITCODE -ne 0) {
        $detail = ($exportOutput | Out-String).Trim()
        if ($detail) {
            throw "secedit export failed (exit $LASTEXITCODE): $detail"
        }
        throw "secedit export failed (exit $LASTEXITCODE)"
    }

    $lines = Get-Content -Path $cfgPath -Encoding Unicode
    $lineIndex = -1
    for ($i = 0; $i -lt $lines.Count; $i++) {
        if ($lines[$i] -match '^SeBatchLogonRight\s*=') {
            $lineIndex = $i
            break
        }
    }

    if ($lineIndex -ge 0) {
        $parts = $lines[$lineIndex].Split("=", 2)
        $value = ""
        if ($parts.Count -gt 1) {
            $value = $parts[1].Trim()
        }
        $entries = @()
        if ($value) {
            $entries = $value.Split(",") | ForEach-Object { $_.Trim() } | Where-Object { $_ }
        }
        if (-not ($entries -contains $sidEntry)) {
            $entries = @($entries + $sidEntry)
            $lines[$lineIndex] = "SeBatchLogonRight = " + ($entries -join ",")
            Set-Content -Path $cfgPath -Value $lines -Encoding Unicode
        }
    } else {
        $lines += "SeBatchLogonRight = $sidEntry"
        Set-Content -Path $cfgPath -Value $lines -Encoding Unicode
    }

    $applyOutput = & secedit.exe /configure /db $dbPath /cfg $cfgPath /areas USER_RIGHTS 2>&1
    if ($LASTEXITCODE -ne 0) {
        $detail = ($applyOutput | Out-String).Trim()
        if ($detail) {
            throw "secedit configure failed (exit $LASTEXITCODE): $detail"
        }
        throw "secedit configure failed (exit $LASTEXITCODE)"
    }
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

$uiTaskName = $null
$userName = $null
$password = $null
$userCreated = $false
$pidPath = $null
$launchScript = $null
$uiPid = $null
$startedViaTask = $false
$taskDir = "C:\\xp2p\\build\\ui-task"

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

    $userName = "xp2pui-" + ([System.Guid]::NewGuid().ToString("N").Substring(0, 8))
    $password = "Xp2pUi!" + (Get-Random -Minimum 100000 -Maximum 999999).ToString() + "Aa"
    $userPath = "$env:COMPUTERNAME\$userName"
    $secure = ConvertTo-SecureString $password -AsPlainText -Force

    $localUserCmd = Get-Command -Name New-LocalUser -ErrorAction SilentlyContinue
    if ($localUserCmd) {
        New-LocalUser -Name $userName -Password $secure -AccountNeverExpires -PasswordNeverExpires:$true | Out-Null
        $userCreated = $true
    } else {
        $userOutput = & net user $userName /add 2>&1
        if ($LASTEXITCODE -ne 0) {
            $detail = ($userOutput | Out-String).Trim()
            if ($detail) {
                throw "Failed to create local user $userName (exit $LASTEXITCODE): $detail"
            }
            throw "Failed to create local user $userName (exit $LASTEXITCODE)"
        }
        $userOutput = & net user $userName "$password" 2>&1
        if ($LASTEXITCODE -ne 0) {
            $detail = ($userOutput | Out-String).Trim()
            if ($detail) {
                throw "Failed to set password for $userName (exit $LASTEXITCODE): $detail"
            }
            throw "Failed to set password for $userName (exit $LASTEXITCODE)"
        }
        $userCreated = $true
    }

    $groupOutput = & net localgroup Users $userName /add 2>&1
    if ($LASTEXITCODE -ne 0) {
        $detail = ($groupOutput | Out-String).Trim()
        if ($detail) {
            throw "Failed to add $userName to Users group (exit $LASTEXITCODE): $detail"
        }
        throw "Failed to add $userName to Users group (exit $LASTEXITCODE)"
    }

    Ensure-BatchLogonRight $userPath

    $credential = New-Object System.Management.Automation.PSCredential($userPath, $secure)
    try {
        $proc = Start-Process -FilePath $Xp2pUiPath -Credential $credential -PassThru
        $uiPid = $proc.Id
    } catch {
        $uiPid = $null
    }

    if (-not $uiPid) {
        New-Item -ItemType Directory -Path $taskDir -Force | Out-Null
        $uiTaskName = "xp2p-ui-log-" + ([System.Guid]::NewGuid().ToString("N").Substring(0, 8))
        $pidPath = Join-Path $taskDir "$uiTaskName.pid.txt"
        $launchScript = Join-Path $taskDir "$uiTaskName.launch.ps1"
        $safePath = $Xp2pUiPath.Replace("'", "''")
        $safePid = $pidPath.Replace("'", "''")
        $scriptBody = @(
            '$ErrorActionPreference = ''Stop'''
            "`$p = Start-Process -FilePath '$safePath' -PassThru"
            "Set-Content -Path '$safePid' -Value `$p.Id -Encoding ASCII"
        ) -join "`r`n"
        Set-Content -Path $launchScript -Value $scriptBody -Encoding ASCII

        $startTime = (Get-Date).AddMinutes(1).ToString("HH:mm")
        $createArgs = @(
            "/Create",
            "/TN", $uiTaskName,
            "/TR", "powershell.exe -NoProfile -ExecutionPolicy Bypass -File `"$launchScript`"",
            "/SC", "ONCE",
            "/ST", $startTime,
            "/RL", "LIMITED",
            "/RU", $userPath,
            "/RP", $password,
            "/F"
        )
        $createOutput = & schtasks.exe @createArgs 2>&1
        if ($LASTEXITCODE -ne 0) {
            $detail = ($createOutput | Out-String).Trim()
            if ($detail) {
                throw "schtasks create failed (exit $LASTEXITCODE): $detail"
            }
            throw "schtasks create failed (exit $LASTEXITCODE)"
        }
        $runOutput = & schtasks.exe /Run /TN $uiTaskName 2>&1
        if ($LASTEXITCODE -ne 0) {
            $detail = ($runOutput | Out-String).Trim()
            if ($detail) {
                throw "schtasks run failed (exit $LASTEXITCODE): $detail"
            }
            throw "schtasks run failed (exit $LASTEXITCODE)"
        }
        $startedViaTask = $true
    }

    if ($startedViaTask) {
        $pidDeadline = [DateTime]::UtcNow.AddSeconds(20)
        while ([DateTime]::UtcNow -lt $pidDeadline) {
            if (Test-Path $pidPath) {
                break
            }
            Start-Sleep -Seconds 1
        }
        if (-not (Test-Path $pidPath)) {
            $exitCode = 4
            $errorDetail = "xp2p-ui did not report pid"
            return
        }
        $uiPid = (Get-Content -Path $pidPath -ErrorAction SilentlyContinue | Select-Object -First 1).Trim()
        if (-not $uiPid) {
            $exitCode = 4
            $errorDetail = "xp2p-ui pid empty"
            return
        }
    }
    if ($WaitSeconds -gt 0) {
        Start-Sleep -Seconds $WaitSeconds
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
    if ($uiTaskName) {
        & schtasks.exe /Delete /TN $uiTaskName /F | Out-Null
    }
    if ($launchScript -and (Test-Path $launchScript)) {
        Remove-Item -Path $launchScript -Force -ErrorAction SilentlyContinue
    }
    if ($pidPath -and (Test-Path $pidPath)) {
        Remove-Item -Path $pidPath -Force -ErrorAction SilentlyContinue
    }
    if ($userName -and $userCreated) {
        $removeCmd = Get-Command -Name Remove-LocalUser -ErrorAction SilentlyContinue
        if ($removeCmd) {
            Remove-LocalUser -Name $userName -ErrorAction SilentlyContinue
        } else {
            & net user $userName /delete | Out-Null
        }
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
