param(
    [Parameter(Mandatory = $true)]
    [string] $MarkerPath,
    [string] $Xp2pUiPath = "",
    [string] $ServiceNamesBase64 = "",
    [int] $UiWaitSeconds = 5,
    [int] $ServiceWaitSeconds = 20
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

try {
    if ($Xp2pUiPath) {
        if (-not (Test-Path $Xp2pUiPath)) {
            Write-Output "xp2p-ui not found at $Xp2pUiPath"
            $exitCode = 3
            return
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
        Stop-Process -Id $uiProc.Id -Force -ErrorAction SilentlyContinue
    }

    $serviceNames = @()
    if ($ServiceNamesBase64) {
        $decoded = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($ServiceNamesBase64))
        $serviceNames = $decoded | ConvertFrom-Json
    }
    if (-not $serviceNames -or $serviceNames.Count -eq 0) {
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

    foreach ($name in $serviceNames) {
        $initial = (Get-Service -Name $name).Status.ToString()
        if ($initial -eq "Running") {
            $lastOperation = "sc.exe stop $name"
            Invoke-ScAsUser $userPath $password "stop" $name
            Wait-ServiceState $name "Stopped" $ServiceWaitSeconds
            $lastOperation = "sc.exe start $name"
            Invoke-ScAsUser $userPath $password "start" $name
            Wait-ServiceState $name "Running" $ServiceWaitSeconds
        } else {
            $lastOperation = "sc.exe start $name"
            Invoke-ScAsUser $userPath $password "start" $name
            Wait-ServiceState $name "Running" $ServiceWaitSeconds
            $lastOperation = "sc.exe stop $name"
            Invoke-ScAsUser $userPath $password "stop" $name
            Wait-ServiceState $name "Stopped" $ServiceWaitSeconds
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
