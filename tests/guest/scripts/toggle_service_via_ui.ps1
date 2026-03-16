param(
    [Parameter(Mandatory = $true)]
    [string] $MarkerPath,
    [string] $Xp2pUiPath = "",
    [string] $ServiceNamesBase64 = "",
    [int] $UiWaitSeconds = 5,
    [int] $ServiceWaitSeconds = 20,
    [string] $LogPath = "C:\\ProgramData\\xp2p\\logs\\xp2p-ui.log",
    [string] $ClearLog = "true",
    [int] $UiPollSeconds = 6,
    [string] $RequiredPatternsBase64 = "",
    [string] $ExpectCrash = "false",
    [int] $CrashWaitSeconds = 60,
    [string] $CrashServiceName = "",
    [string] $ConfigDirsBase64 = "",
    [string] $StartStatusesBase64 = "",
    [string] $StatusPollSeconds = "",
    [string] $AllowStoppedOnly = "false"
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

function Decode-JsonList([string] $payload) {
    if (-not $payload) {
        return @()
    }
    $decoded = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($payload))
    if (-not $decoded) {
        return @()
    }
    $data = $decoded | ConvertFrom-Json
    if ($data -is [System.Array]) {
        return @($data | ForEach-Object { $_.ToString() })
    }
    return @($data.ToString())
}

function Wait-ForLog([string] $path, [int] $timeoutSeconds) {
    $deadline = [DateTime]::UtcNow.AddSeconds($timeoutSeconds)
    while ([DateTime]::UtcNow -lt $deadline) {
        if (Test-Path $path) {
            return
        }
        Start-Sleep -Seconds 1
    }
}

function Get-LogText([string] $path, [int] $maxLines) {
    if (-not (Test-Path $path)) {
        return ""
    }
    $tail = Get-Content -Path $path -ErrorAction SilentlyContinue -Tail $maxLines
    return ($tail | Out-String)
}

function Get-ServiceKey([string] $serviceName) {
    $lower = $serviceName.ToLowerInvariant()
    if ($lower -like "*client*") {
        return "client"
    }
    if ($lower -like "*server*") {
        return "server"
    }
    return $serviceName
}

function Has-StatusSequence([string] $logText, [string] $serviceKey, [string[]] $startStatuses) {
    if (-not $logText) {
        return $false
    }
    $startStatuses = @($startStatuses)
    if ($startStatuses.Count -eq 0) {
        $startStatuses = @("Running")
    }
    $startAlt = ($startStatuses | ForEach-Object { [regex]::Escape($_) }) -join "|"
    $startPattern = "tray status: .*${serviceKey}=(${startAlt})"
    $stoppedPattern = "tray status: .*${serviceKey}=Stopped"
    $lines = $logText -split "(`r`n|`r|`n)"
    $runningIndex = -1
    for ($i = 0; $i -lt $lines.Count; $i++) {
        $line = $lines[$i]
        if ($runningIndex -lt 0 -and $line -match $startPattern) {
            $runningIndex = $i
            continue
        }
        if ($runningIndex -ge 0 -and $line -match $stoppedPattern) {
            return $true
        }
    }
    return $false
}

function Has-StoppedStatus([string] $logText, [string] $serviceKey) {
    if (-not $logText) {
        return $false
    }
    $stoppedPattern = "tray status: .*${serviceKey}=Stopped"
    return ($logText -match $stoppedPattern)
}

function Wait-ForStatusSequence([string] $path, [string] $serviceKey, [string[]] $startStatuses, [bool] $allowStoppedOnly, [int] $timeoutSeconds) {
    $deadline = [DateTime]::UtcNow.AddSeconds($timeoutSeconds)
    while ([DateTime]::UtcNow -lt $deadline) {
        $logText = Get-LogText $path 800
        if (Has-StatusSequence $logText $serviceKey $startStatuses) {
            return
        }
        if ($allowStoppedOnly -and (Has-StoppedStatus $logText $serviceKey)) {
            return
        }
        Start-Sleep -Seconds 2
    }
    throw "log sequence not found for $serviceKey within ${timeoutSeconds}s"
}

function Wait-ServiceState([string] $name, [string] $expected, [int] $timeoutSeconds) {
    $deadline = [DateTime]::UtcNow.AddSeconds($timeoutSeconds)
    while ([DateTime]::UtcNow -lt $deadline) {
        $svc = Get-Service -Name $name -ErrorAction SilentlyContinue
        if (-not $svc) {
            throw "Service $name not found"
        }
        if ($svc.Status.ToString().ToLowerInvariant() -eq $expected.ToLowerInvariant()) {
            return
        }
        Start-Sleep -Seconds 1
    }
    throw "Service $name did not reach state $expected within ${timeoutSeconds}s"
}

function Invoke-ScAsUser([string] $userPath, [string] $password, [string] $action, [string] $serviceName) {
    $safeArgs = @(
        $action,
        $serviceName
    ) | Where-Object { $_ -ne $null -and $_.ToString().Trim() -ne "" }
    if (-not $safeArgs -or $safeArgs.Count -lt 2) {
        throw "sc.exe received invalid arguments (action=$action, service=$serviceName)"
    }
    $scPath = Join-Path $env:SystemRoot "System32\\sc.exe"
    $taskName = "xp2p-ui-svc-" + ([System.Guid]::NewGuid().ToString("N").Substring(0, 8))
    $taskDir = "C:\\xp2p\\build\\ui-task"
    New-Item -ItemType Directory -Path $taskDir -Force | Out-Null
    $taskOutput = Join-Path $taskDir "$taskName.out.txt"
    $taskRc = Join-Path $taskDir "$taskName.rc.txt"
    $cmd = "`"$scPath`" $($safeArgs -join ' ') > `"$taskOutput`" 2>&1 & echo %errorlevel% > `"$taskRc`""
    $taskCmd = "cmd.exe /c `"$cmd`""

    $startTime = (Get-Date).AddMinutes(1).ToString("HH:mm")
    $createArgs = @(
        "/Create",
        "/TN", $taskName,
        "/TR", $taskCmd,
        "/SC", "ONCE",
        "/ST", $startTime,
        "/RL", "LIMITED",
        "/RU", $userPath,
        "/RP", $password,
        "/F"
    )
    $createOutput = & schtasks.exe @createArgs 2>&1
    $createCode = $LASTEXITCODE
    if ($createCode -ne 0) {
        $detail = ($createOutput | Out-String).Trim()
        if ($detail) {
            throw "schtasks create failed with exit code $createCode for args: $($safeArgs -join ' '); $detail"
        }
        throw "schtasks create failed with exit code $createCode for args: $($safeArgs -join ' ')"
    }

    $runOutput = & schtasks.exe /Run /TN $taskName 2>&1
    $runCode = $LASTEXITCODE
    if ($runCode -ne 0) {
        $detail = ($runOutput | Out-String).Trim()
        if ($detail) {
            throw "schtasks run failed with exit code $runCode for args: $($safeArgs -join ' '); $detail"
        }
        throw "schtasks run failed with exit code $runCode for args: $($safeArgs -join ' ')"
    }

    $deadline = [DateTime]::UtcNow.AddSeconds(30)
    do {
        if (Test-Path $taskRc) {
            break
        }
        Start-Sleep -Seconds 1
    } while ([DateTime]::UtcNow -lt $deadline)

    & schtasks.exe /Delete /TN $taskName /F | Out-Null

    if (-not (Test-Path $taskRc)) {
        throw "scheduled task did not write result for args: $($safeArgs -join ' ')"
    }
    $result = (Get-Content -Path $taskRc -ErrorAction SilentlyContinue | Select-Object -First 1).Trim()
    if ($result -ne "0") {
        $outputText = ""
        if (Test-Path $taskOutput) {
            $outputText = (Get-Content -Path $taskOutput -ErrorAction SilentlyContinue | Out-String).Trim()
        }
        if ($outputText) {
            throw "sc.exe failed with exit $result for args: $($safeArgs -join ' '); $outputText"
        }
        throw "sc.exe failed with exit $result for args: $($safeArgs -join ' ')"
    }

    Remove-Item -Path $taskRc, $taskOutput -ErrorAction SilentlyContinue
    & schtasks.exe /Delete /TN $taskName /F | Out-Null
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

$exitCode = 0
$errorDetail = ""
$lastOperation = ""
$userName = $null
$password = $null
$userCreated = $false
$uiProc = $null
$expectCrashFlag = $false

try {
    if ($ExpectCrash) {
        if ($ExpectCrash -is [string]) {
            $expectCrashFlag = ($ExpectCrash.ToLowerInvariant() -eq "true")
        } else {
            $expectCrashFlag = [bool]$ExpectCrash
        }
    }
    if ($ClearLog -and $ClearLog.ToLowerInvariant() -ne "false") {
        $logDir = Split-Path -Parent $LogPath
        if ($logDir) {
            New-Item -ItemType Directory -Path $logDir -Force | Out-Null
        }
        if (Test-Path $LogPath) {
            Remove-Item -Path $LogPath -Force -ErrorAction SilentlyContinue
        }
    }

    if ($Xp2pUiPath) {
        if (-not (Test-Path $Xp2pUiPath)) {
            Write-Output "xp2p-ui not found at $Xp2pUiPath"
            $exitCode = 3
            return
        }
        if ($StatusPollSeconds) {
            $env:XP2P_UI_STATUS_POLL_SECONDS = $StatusPollSeconds
        }
        $uiProc = Start-Process -FilePath $Xp2pUiPath -PassThru
        if ($UiWaitSeconds -gt 0) {
            Start-Sleep -Seconds $UiWaitSeconds
        }
        $uiRunning = Get-Process -Id $uiProc.Id -ErrorAction SilentlyContinue
        if (-not $uiRunning) {
            Write-Output "xp2p-ui exited before service toggle"
            $exitCode = 4
            return
        }
        Wait-ForLog $LogPath 10
    }

    $serviceNames = @()
    if ($ServiceNamesBase64) {
        $serviceNames = Decode-JsonList $ServiceNamesBase64
    }
    $serviceNames = @($serviceNames)
    if ($serviceNames.Count -eq 0) {
        $serviceNames = @("xp2p-client", "xp2p-server")
    }
    $serviceNames = @($serviceNames | ForEach-Object { $_.ToString().Trim() } | Where-Object { $_ })

    foreach ($name in $serviceNames) {
        $svc = Get-Service -Name $name -ErrorAction SilentlyContinue
        if (-not $svc) {
            Write-Output "Service $name not found"
            $exitCode = 5
            return
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
    Ensure-BatchLogonRight $userPath

    if ($expectCrashFlag) {
        $targetName = $CrashServiceName
        if (-not $targetName) {
            $targetName = $serviceNames | Select-Object -First 1
        }
        if (-not $targetName) {
            throw "No service name available for crash test"
        }
        $configDirs = @((Decode-JsonList $ConfigDirsBase64))
        foreach ($dir in $configDirs) {
            if ($dir -and (Test-Path $dir)) {
                Remove-Item -Path $dir -Recurse -Force -ErrorAction SilentlyContinue
            }
        }
        $startStatuses = @((Decode-JsonList $StartStatusesBase64))
        $allowStoppedOnlyFlag = $false
        if ($AllowStoppedOnly) {
            if ($AllowStoppedOnly -is [string]) {
                $allowStoppedOnlyFlag = ($AllowStoppedOnly.ToLowerInvariant() -eq "true")
            } else {
                $allowStoppedOnlyFlag = [bool]$AllowStoppedOnly
            }
        }
        $lastOperation = "sc.exe start $targetName"
        Invoke-ScAsUser $userPath $password "start" $targetName
        if ($UiPollSeconds -gt 0) {
            Start-Sleep -Seconds $UiPollSeconds
        }
        $serviceKey = Get-ServiceKey $targetName
        Wait-ForStatusSequence $LogPath $serviceKey $startStatuses $allowStoppedOnlyFlag $CrashWaitSeconds
        Wait-ServiceState $targetName "Stopped" $CrashWaitSeconds
    } else {
        foreach ($name in $serviceNames) {
            $initial = (Get-Service -Name $name).Status.ToString()
            if ($initial -eq "Running") {
                $lastOperation = "sc.exe stop $name"
                Invoke-ScAsUser $userPath $password "stop" $name
                Wait-ServiceState $name "Stopped" $ServiceWaitSeconds
                if ($UiPollSeconds -gt 0) {
                    Start-Sleep -Seconds $UiPollSeconds
                }
                $lastOperation = "sc.exe start $name"
                Invoke-ScAsUser $userPath $password "start" $name
                Wait-ServiceState $name "Running" $ServiceWaitSeconds
                if ($UiPollSeconds -gt 0) {
                    Start-Sleep -Seconds $UiPollSeconds
                }
            } else {
                $lastOperation = "sc.exe start $name"
                Invoke-ScAsUser $userPath $password "start" $name
                Wait-ServiceState $name "Running" $ServiceWaitSeconds
                if ($UiPollSeconds -gt 0) {
                    Start-Sleep -Seconds $UiPollSeconds
                }
                $lastOperation = "sc.exe stop $name"
                Invoke-ScAsUser $userPath $password "stop" $name
                Wait-ServiceState $name "Stopped" $ServiceWaitSeconds
                if ($UiPollSeconds -gt 0) {
                    Start-Sleep -Seconds $UiPollSeconds
                }
            }
        }
    }

    $patterns = @((Decode-JsonList $RequiredPatternsBase64))
    if ($patterns.Count -gt 0) {
        $logText = Get-LogText $LogPath 400
        foreach ($pattern in $patterns) {
            if (-not ($logText -match $pattern)) {
                throw "log pattern not found: $pattern"
            }
        }
    }
} catch {
    $errorDetail = ($_ | Out-String).Trim()
    if ($lastOperation) {
        $errorDetail = "LastOp=$lastOperation; $errorDetail"
    }
    Write-Output "toggle_service_via_ui failed: $errorDetail"
    $exitCode = 10
} finally {
    if ($uiProc -and -not $uiProc.HasExited) {
        Stop-Process -Id $uiProc.Id -Force -ErrorAction SilentlyContinue
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
