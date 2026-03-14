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
    $taskCmd = "`"$scPath $($safeArgs -join ' ')`""

    $createArgs = @(
        "/Create",
        "/TN", $taskName,
        "/TR", $taskCmd,
        "/SC", "ONCE",
        "/ST", "00:00",
        "/RL", "LIMITED",
        "/RU", $userPath,
        "/RP", $password,
        "/F"
    )
    $createProc = Start-Process -FilePath "schtasks.exe" -ArgumentList $createArgs -PassThru -Wait -NoNewWindow
    if ($createProc.ExitCode -ne 0) {
        throw "schtasks create failed with exit code $($createProc.ExitCode) for args: $($safeArgs -join ' ')"
    }

    $runProc = Start-Process -FilePath "schtasks.exe" -ArgumentList @("/Run", "/TN", $taskName) -PassThru -Wait -NoNewWindow
    if ($runProc.ExitCode -ne 0) {
        throw "schtasks run failed with exit code $($runProc.ExitCode) for args: $($safeArgs -join ' ')"
    }

    $deadline = [DateTime]::UtcNow.AddSeconds(30)
    $taskInfo = $null
    do {
        $taskInfo = Get-ScheduledTaskInfo -TaskName $taskName -ErrorAction SilentlyContinue
        if ($taskInfo -and $taskInfo.State -ne "Running") {
            break
        }
        Start-Sleep -Seconds 1
    } while ([DateTime]::UtcNow -lt $deadline)

    & schtasks.exe /Delete /TN $taskName /F | Out-Null

    if (-not $taskInfo) {
        throw "scheduled task info missing for args: $($safeArgs -join ' ')"
    }
    if ($taskInfo.LastTaskResult -ne 0) {
        throw "sc.exe failed with result $($taskInfo.LastTaskResult) for args: $($safeArgs -join ' ')"
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
