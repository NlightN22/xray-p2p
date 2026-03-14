param(
    [Parameter(Mandatory = $true)]
    [string] $MarkerPath,
    [string] $ServiceNames = "xp2p-client,xp2p-server",
    [string] $OutputDir = "C:\\xp2p\\build\\ui-sc",
    [string] $UserName = "vagrant",
    [string] $UserPassword = "vagrant",
    [string] $UseExistingUser = "true",
    [string] $GrantLogonRights = "false"
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

function Convert-ToBool([string] $value, [bool] $defaultValue) {
    if ($null -eq $value) {
        return $defaultValue
    }
    $text = $value.Trim().ToLowerInvariant()
    if ($text -in @("1", "true", "yes", "y")) {
        return $true
    }
    if ($text -in @("0", "false", "no", "n")) {
        return $false
    }
    return $defaultValue
}

function Ensure-UserRight([string] $userPath, [string] $rightName) {
    $account = New-Object System.Security.Principal.NTAccount($userPath)
    $sid = $account.Translate([System.Security.Principal.SecurityIdentifier]).Value
    if (-not $sid) {
        throw "Unable to resolve SID for $userPath"
    }
    $sidEntry = "*$sid"

    $policyDir = "C:\\xp2p\\build\\ui-sc"
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
        if ($lines[$i] -match ("^" + [regex]::Escape($rightName) + "\\s*=")) {
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
            $lines[$lineIndex] = "$rightName = " + ($entries -join ",")
            Set-Content -Path $cfgPath -Value $lines -Encoding Unicode
        }
    } else {
        $lines += "$rightName = $sidEntry"
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

function Get-DenyLogonInfo([string] $userPath) {
    $account = New-Object System.Security.Principal.NTAccount($userPath)
    $sid = $account.Translate([System.Security.Principal.SecurityIdentifier]).Value
    if (-not $sid) {
        return ""
    }
    $sidEntry = "*$sid"
    $policyDir = "C:\\xp2p\\build\\ui-sc"
    New-Item -ItemType Directory -Path $policyDir -Force | Out-Null
    $cfgPath = Join-Path $policyDir ("secpol-deny-{0}-{1}.cfg" -f (Get-Date -Format "yyyyMMddHHmmss"), ([System.Guid]::NewGuid().ToString("N").Substring(0, 6)))
    $exportOutput = & secedit.exe /export /cfg $cfgPath /areas USER_RIGHTS 2>&1
    if ($LASTEXITCODE -ne 0) {
        return ""
    }
    $lines = Get-Content -Path $cfgPath -Encoding Unicode
    $denyRights = @("SeDenyBatchLogonRight", "SeDenyInteractiveLogonRight")
    $hits = @()
    foreach ($right in $denyRights) {
        $line = $lines | Where-Object { $_ -match ("^" + [regex]::Escape($right) + "\\s*=") } | Select-Object -First 1
        if (-not $line) {
            continue
        }
        $parts = $line.Split("=", 2)
        if ($parts.Count -lt 2) {
            continue
        }
        $entries = $parts[1].Trim().Split(",") | ForEach-Object { $_.Trim() } | Where-Object { $_ }
        if ($entries -contains $sidEntry) {
            $hits += $right
        }
    }
    if ($hits.Count -gt 0) {
        return ($hits -join ",")
    }
    return ""
}

$exitCode = 0
$errorDetail = ""
$accountName = $null
$userCreated = $false
$denyInfo = ""

try {
    if (-not $ServiceNames) {
        $exitCode = 2
        $errorDetail = "ServiceNames is empty"
        return
    }

    $services = $ServiceNames.Split(",") | ForEach-Object { $_.Trim() } | Where-Object { $_ }
    if (-not $services) {
        $exitCode = 2
        $errorDetail = "ServiceNames is empty"
        return
    }

    $useExistingUser = Convert-ToBool $UseExistingUser $true
    $grantLogonRights = Convert-ToBool $GrantLogonRights $false

    if ($useExistingUser) {
        if (-not $UserName -or -not $UserPassword) {
            throw "UserName and UserPassword are required when UseExistingUser=true"
        }
        $accountName = $UserName
        $password = $UserPassword
        $userPath = "$env:COMPUTERNAME\\$accountName"
        $secure = ConvertTo-SecureString $password -AsPlainText -Force
        if ($grantLogonRights) {
            Ensure-UserRight $userPath "SeBatchLogonRight"
            Ensure-UserRight $userPath "SeInteractiveLogonRight"
            $prevPref = $ErrorActionPreference
            $ErrorActionPreference = 'Continue'
            & gpupdate.exe /target:user /force | Out-Null
            $ErrorActionPreference = $prevPref
        }
        $denyInfo = Get-DenyLogonInfo $userPath
    } else {
        $accountName = "xp2psc-" + ([System.Guid]::NewGuid().ToString("N").Substring(0, 8))
        $password = "Xp2pSc!" + (Get-Random -Minimum 100000 -Maximum 999999).ToString() + "Aa"
        $userPath = "$env:COMPUTERNAME\\$accountName"
        $secure = ConvertTo-SecureString $password -AsPlainText -Force

        $localUserCmd = Get-Command -Name New-LocalUser -ErrorAction SilentlyContinue
        if ($localUserCmd) {
            New-LocalUser -Name $accountName -Password $secure -AccountNeverExpires -PasswordNeverExpires:$true | Out-Null
            $userCreated = $true
        } else {
            $userOutput = & net user $accountName /add 2>&1
            if ($LASTEXITCODE -ne 0) {
                $detail = ($userOutput | Out-String).Trim()
                if ($detail) {
                    throw "Failed to create local user $accountName (exit $LASTEXITCODE): $detail"
                }
                throw "Failed to create local user $accountName (exit $LASTEXITCODE)"
            }
            $userOutput = & net user $accountName "$password" 2>&1
            if ($LASTEXITCODE -ne 0) {
                $detail = ($userOutput | Out-String).Trim()
                if ($detail) {
                    throw "Failed to set password for $accountName (exit $LASTEXITCODE): $detail"
                }
                throw "Failed to set password for $accountName (exit $LASTEXITCODE)"
            }
            $userCreated = $true
        }

        $groupOutput = & net localgroup Users $accountName /add 2>&1
        if ($LASTEXITCODE -ne 0) {
            $detail = ($groupOutput | Out-String).Trim()
            if ($detail) {
                throw "Failed to add $accountName to Users group (exit $LASTEXITCODE): $detail"
            }
            throw "Failed to add $accountName to Users group (exit $LASTEXITCODE)"
        }

        Ensure-UserRight $userPath "SeBatchLogonRight"
        Ensure-UserRight $userPath "SeInteractiveLogonRight"
        $prevPref = $ErrorActionPreference
        $ErrorActionPreference = 'Continue'
        & gpupdate.exe /target:user /force | Out-Null
        $ErrorActionPreference = $prevPref
        $denyInfo = Get-DenyLogonInfo $userPath
    }

    [System.IO.Directory]::CreateDirectory($OutputDir) | Out-Null
    $token = [System.Guid]::NewGuid().ToString("N").Substring(0, 8)
    $stdOutPath = Join-Path $OutputDir ("sc-query-{0}.stdout.txt" -f $token)
    $stdErrPath = Join-Path $OutputDir ("sc-query-{0}.stderr.txt" -f $token)
    $cmdLine = ($services | ForEach-Object { "echo -- $_ & sc.exe query $_" }) -join " & "

    $credential = New-Object System.Management.Automation.PSCredential($userPath, $secure)
    $proc = Start-Process -FilePath "cmd.exe" -ArgumentList @(
        "/c",
        $cmdLine
    ) -Credential $credential -PassThru -Wait -RedirectStandardOutput $stdOutPath -RedirectStandardError $stdErrPath
    if ($proc.ExitCode -ne 0) {
        $exitCode = 5
        $detailParts = @("sc query failed with exit code $($proc.ExitCode)")
        if (Test-Path $stdErrPath) {
            $stderrText = (Get-Content -Path $stdErrPath -ErrorAction SilentlyContinue | Out-String).Trim()
            if ($stderrText) {
                $detailParts += ("stderr=" + $stderrText)
            }
        }
        if (Test-Path $stdOutPath) {
            $stdoutText = (Get-Content -Path $stdOutPath -ErrorAction SilentlyContinue | Out-String).Trim()
            if ($stdoutText) {
                $detailParts += ("stdout=" + $stdoutText)
            }
        }
        if ($denyInfo) {
            $detailParts += ("deny_rights=" + $denyInfo)
        }
        $taskName = "xp2p-sc-" + ([System.Guid]::NewGuid().ToString("N").Substring(0, 8))
        $taskCmd = Join-Path $OutputDir ("sc-query-{0}.cmd" -f $token)
        $taskOut = Join-Path $OutputDir ("sc-query-{0}.task-stdout.txt" -f $token)
        $taskErr = Join-Path $OutputDir ("sc-query-{0}.task-stderr.txt" -f $token)
        $taskExit = Join-Path $OutputDir ("sc-query-{0}.task-exit.txt" -f $token)
        $cmdLines = @(
            "@echo off",
            "cmd.exe /c $cmdLine 1> ""$taskOut"" 2> ""$taskErr""",
            "echo %ERRORLEVEL%> ""$taskExit"""
        )
        Set-Content -Path $taskCmd -Value $cmdLines -Encoding ASCII

        $startTime = (Get-Date).AddMinutes(1).ToString("HH:mm")
        $createArgs = @(
            "/Create",
            "/TN", $taskName,
            "/TR", "`"$taskCmd`"",
            "/SC", "ONCE",
            "/ST", $startTime,
            "/RL", "LIMITED",
            "/RU", $userPath,
            "/RP", $password,
            "/F"
        )
        $prevPref = $ErrorActionPreference
        $ErrorActionPreference = 'Continue'
        $createOutput = & schtasks.exe @createArgs 2>&1
        $ErrorActionPreference = $prevPref
        if ($LASTEXITCODE -eq 0) {
            $runOutput = & schtasks.exe /Run /TN $taskName 2>&1
            $taskDeadline = [DateTime]::UtcNow.AddSeconds(30)
            while ([DateTime]::UtcNow -lt $taskDeadline) {
                if (Test-Path $taskExit) {
                    break
                }
                Start-Sleep -Seconds 1
            }
            if (Test-Path $taskExit) {
                $taskExitCode = (Get-Content -Path $taskExit -ErrorAction SilentlyContinue | Select-Object -First 1).Trim()
                if ($taskExitCode) {
                    $detailParts += ("task_exit=" + $taskExitCode)
                }
            } else {
                $detailParts += "task_exit=<missing>"
            }
            if (Test-Path $taskErr) {
                $taskErrText = (Get-Content -Path $taskErr -ErrorAction SilentlyContinue | Out-String).Trim()
                if ($taskErrText) {
                    $detailParts += ("task_stderr=" + $taskErrText)
                }
            }
            if (Test-Path $taskOut) {
                $taskOutText = (Get-Content -Path $taskOut -ErrorAction SilentlyContinue | Out-String).Trim()
                if ($taskOutText) {
                    $detailParts += ("task_stdout=" + $taskOutText)
                }
            }
        } else {
            $detailParts += "task_create_failed"
        }
        if ($createOutput) {
            $createText = ($createOutput | Out-String).Trim()
            if ($createText) {
                $detailParts += ("task_create_out=" + $createText)
            }
        }
        if ($taskName) {
            & schtasks.exe /Delete /TN $taskName /F | Out-Null
        }
        $errorDetail = $detailParts -join " | "
        return
    }

    if (-not (Test-Path $stdOutPath)) {
        $exitCode = 6
        $errorDetail = "sc query output missing at $stdOutPath"
        return
    }
} catch {
    $exitCode = 10
    $errorDetail = ($_ | Out-String).Trim()
} finally {
    if ($accountName -and $userCreated) {
        $removeCmd = Get-Command -Name Remove-LocalUser -ErrorAction SilentlyContinue
        if ($removeCmd) {
            Remove-LocalUser -Name $accountName -ErrorAction SilentlyContinue
        } else {
            & net user $accountName /delete | Out-Null
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
