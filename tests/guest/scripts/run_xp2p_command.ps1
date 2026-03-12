param(
    [Parameter(Mandatory = $true)]
    [string]$Xp2pPath,
    [Parameter(Mandatory = $true)]
    [string]$ArgsBase64,
    [string]$TimeoutPath,
    [int]$TimeoutSeconds = 120
)

$ErrorActionPreference = 'Stop'

if (-not (Test-Path $Xp2pPath)) {
    Write-Output '__XP2P_MISSING__'
    exit 3
}

try {
    $decoded = [System.Text.Encoding]::UTF8.GetString(
        [System.Convert]::FromBase64String($ArgsBase64)
    )
} catch {
    throw "Failed to decode xp2p argument payload. Error: $($_.Exception.Message)"
}

try {
    $arguments = ConvertFrom-Json -InputObject $decoded -ErrorAction Stop
} catch {
    throw "Failed to parse xp2p argument payload. Error: $($_.Exception.Message)"
}

if (-not ($arguments -is [System.Collections.IEnumerable])) {
    $arguments = @()
}

$runtimeRoot = Join-Path $env:WINDIR 'Temp\xp2p-runner'
if (-not (Test-Path $runtimeRoot)) {
    New-Item -ItemType Directory -Path $runtimeRoot -Force | Out-Null
}

$taskId = [guid]::NewGuid().ToString()
$taskName = "xp2p-tests-$taskId"
$scriptPath = Join-Path $runtimeRoot "$taskId.ps1"
$outPath = Join-Path $runtimeRoot "$taskId.out"
$errPath = Join-Path $runtimeRoot "$taskId.err"
$codePath = Join-Path $runtimeRoot "$taskId.code"
$timingPath = Join-Path $runtimeRoot "$taskId.timing"
$psexecOutPath = Join-Path $runtimeRoot "$taskId.psexec.out"
$psexecErrPath = Join-Path $runtimeRoot "$taskId.psexec.err"
$timeoutSeconds = $TimeoutSeconds

function Remove-TemporaryFile {
    param([string]$Path)
    if ($Path -and (Test-Path $Path)) {
        Remove-Item -Path $Path -Force -ErrorAction SilentlyContinue
    }
}

function Cleanup {
    Remove-TemporaryFile -Path $scriptPath
    Remove-TemporaryFile -Path $outPath
    Remove-TemporaryFile -Path $errPath
    Remove-TemporaryFile -Path $codePath
    Remove-TemporaryFile -Path $timingPath
    Remove-TemporaryFile -Path $psexecOutPath
    Remove-TemporaryFile -Path $psexecErrPath
}

function Set-TimeoutMarker {
    param([string]$Message)
    if (-not $TimeoutPath) {
        return
    }
    $dir = Split-Path -Parent $TimeoutPath
    if ($dir -and -not (Test-Path $dir)) {
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
    }
    Set-Content -Path $TimeoutPath -Value $Message -Encoding ASCII
}

function Stop-Xp2pProcesses {
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
}

function Resolve-PsExecPath {
    $candidate = Get-Command -Name psexec.exe -ErrorAction SilentlyContinue
    if ($candidate -and $candidate.Path) {
        return $candidate.Path
    }
    $fallbacks = @(
        "$env:ProgramData\chocolatey\bin\psexec.exe",
        "$env:ProgramData\chocolatey\lib\sysinternals\tools\PsExec.exe",
        "C:\Sysinternals\PsExec.exe"
    )
    foreach ($path in $fallbacks) {
        if ($path -and (Test-Path $path)) {
            return $path
        }
    }
    return $null
}

function Read-OptionalText {
    param([string]$Path)
    if ($Path -and (Test-Path $Path)) {
        return (Get-Content -Path $Path -Raw)
    }
    return ""
}

$escapedXp2p = $Xp2pPath -replace "'", "''"
$escapedArgsBase64 = $ArgsBase64 -replace "'", "''"

$taskScript = @"
`$ErrorActionPreference = 'Stop'
`$xp2p = '$escapedXp2p'
`$encodedArgs = '$escapedArgsBase64'
if (-not (Test-Path `$xp2p)) {
    Write-Output '__XP2P_MISSING__'
    exit 3
}
function Format-Xp2pArgument {
    param([string]`$Value)
    if ([string]::IsNullOrEmpty(`$Value)) {
        return '""'
    }
    if (`$Value -notmatch '[\s"]') {
        return `$Value
    }
    `$escaped = `$Value -replace '"', '\"'
    return '"{0}"' -f `$escaped
}
`$arguments = @()
if (`$encodedArgs) {
    try {
        `$decoded = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String(`$encodedArgs))
        if (`$decoded) {
            `$arguments = ConvertFrom-Json -InputObject `$decoded
        }
    } catch {
        `$\_.Exception.Message | Out-File -FilePath '$errPath' -Encoding UTF8
        Set-Content -Path '$codePath' -Value 1 -Encoding ASCII
        exit 1
    }
}
try {
`$formatted = @()
    foreach (`$arg in `$arguments) {
        `$formatted += Format-Xp2pArgument -Value (`$arg -as [string])
    }
    `$argumentLiteral = [string]::Join(' ', `$formatted)
    `$start = Get-Date
    `$process = Start-Process -FilePath `$xp2p -ArgumentList `$argumentLiteral -Wait -PassThru -WindowStyle Hidden -RedirectStandardOutput '$outPath' -RedirectStandardError '$errPath'
    `$code = `$process.ExitCode
    `$end = Get-Date
    `$elapsed = `$end - `$start
    "start=`$start; end=`$end; elapsed_ms=`$([int]`$elapsed.TotalMilliseconds)" | Out-File -FilePath '$timingPath' -Encoding ASCII
} catch {
    `$\_.Exception.Message | Out-File -FilePath '$errPath' -Encoding UTF8
    `$code = 1
}
Set-Content -Path '$codePath' -Value `$code -Encoding ASCII
"@

Set-Content -Path $scriptPath -Value $taskScript -Encoding UTF8

if ($TimeoutPath -and (Test-Path $TimeoutPath)) {
    Remove-Item -Path $TimeoutPath -Force -ErrorAction SilentlyContinue
}

$adminLaunch = ""
if ($env:XP2P_ADMIN_LAUNCH) {
    $adminLaunch = $env:XP2P_ADMIN_LAUNCH.ToLowerInvariant()
}
$psExecPath = Resolve-PsExecPath
$requirePsExec = $false
$usePsExec = $false
if ($adminLaunch -eq "psexec") {
    $usePsExec = $true
    $requirePsExec = $true
} elseif ($adminLaunch -eq "scheduled") {
    $usePsExec = $false
} elseif ($psExecPath) {
    $usePsExec = $true
}

$usedScheduledTask = $false
$usedPsExec = $false
$psexecDiagnostics = ""

if ($usePsExec) {
    if (-not $psExecPath) {
        throw "XP2P_ADMIN_LAUNCH=psexec but PsExec was not found on the guest."
    }
    $psexecStart = Get-Date
    $psexecArgs = @(
        "-accepteula",
        "-nobanner",
        "-s",
        "-h",
        "powershell.exe",
        "-NoProfile",
        "-NonInteractive",
        "-ExecutionPolicy",
        "Bypass",
        "-File",
        $scriptPath
    )
    $process = Start-Process -FilePath $psExecPath -ArgumentList $psexecArgs -PassThru -NoNewWindow `
        -RedirectStandardOutput $psexecOutPath -RedirectStandardError $psexecErrPath
    $waitStopwatch = [System.Diagnostics.Stopwatch]::StartNew()
    while (-not $process.HasExited) {
        if ($waitStopwatch.Elapsed.TotalSeconds -gt $timeoutSeconds) {
            Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
            Stop-Xp2pProcesses
            Set-TimeoutMarker -Message "__XP2P_TIMEOUT__"
            Write-Output '__XP2P_TIMEOUT__'
            Cleanup
            exit 124
        }
        Start-Sleep -Milliseconds 200
    }
    $usedPsExec = $true
    $psexecEnd = Get-Date
    if ($process.ExitCode -ne 0 -and -not (Test-Path $codePath)) {
        $psexecDiagnostics = (Read-OptionalText -Path $psexecErrPath).Trim()
        if (-not $psexecDiagnostics) {
            $psexecDiagnostics = (Read-OptionalText -Path $psexecOutPath).Trim()
        }
        if ($requirePsExec) {
            throw "PsExec launch failed (exit $($process.ExitCode)).`n$psexecDiagnostics"
        }
        $usedPsExec = $false
    } else {
        $psexecMs = [int](($psexecEnd - $psexecStart).TotalMilliseconds)
        Write-Output "TIMING: psexec_ms=$psexecMs"
    }
}

if (-not $usedPsExec) {
    $usedScheduledTask = $true
    $parentPid = $PID
    $watchdog = Start-Job -ScriptBlock {
        param($parentPidValue, $taskNameValue, $timeoutValue)
        Start-Sleep -Seconds $timeoutValue
        try { schtasks.exe /End /TN $taskNameValue | Out-Null } catch { }
        try { Get-Process -Name xp2p,xray -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue } catch { }
        try { Stop-Process -Id $parentPidValue -Force -ErrorAction SilentlyContinue } catch { }
    } -ArgumentList $parentPid, $taskName, ($timeoutSeconds + 30)

    Import-Module ScheduledTasks -ErrorAction SilentlyContinue | Out-Null

    try {
        $scheduleStart = Get-Date
        $action = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument "-NoProfile -NonInteractive -ExecutionPolicy Bypass -File `"$scriptPath`""
        $trigger = New-ScheduledTaskTrigger -Once -At ((Get-Date).AddMinutes(2))
        Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger -RunLevel Highest -Force -User 'SYSTEM' | Out-Null
        Start-ScheduledTask -TaskName $taskName

        $waitStart = Get-Date
        $waitStopwatch = [System.Diagnostics.Stopwatch]::StartNew()
        while (-not (Test-Path $codePath)) {
            if ($waitStopwatch.Elapsed.TotalSeconds -gt $timeoutSeconds) {
                Stop-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
                Stop-Xp2pProcesses
                Set-TimeoutMarker -Message "__XP2P_TIMEOUT__"
                Write-Output '__XP2P_TIMEOUT__'
                Cleanup
                exit 124
            }
            Start-Sleep -Seconds 1
        }
        $waitEnd = Get-Date
    } finally {
        Unregister-ScheduledTask -TaskName $taskName -Confirm:$false -ErrorAction SilentlyContinue | Out-Null
        if ($watchdog) {
            Stop-Job -Job $watchdog -ErrorAction SilentlyContinue | Out-Null
            Remove-Job -Job $watchdog -ErrorAction SilentlyContinue | Out-Null
        }
    }
}

if (-not (Test-Path $codePath)) {
    $codeStopwatch = [System.Diagnostics.Stopwatch]::StartNew()
    while (-not (Test-Path $codePath) -and $codeStopwatch.Elapsed.TotalSeconds -lt 10) {
        Start-Sleep -Milliseconds 500
    }
}

if (-not (Test-Path $codePath)) {
    Write-Output "__XP2P_NOEXIT__"
    Set-TimeoutMarker -Message "__XP2P_NOEXIT__"
    if ($usedScheduledTask) {
        $taskInfo = $null
        try {
            $taskInfo = Get-ScheduledTaskInfo -TaskName $taskName -ErrorAction SilentlyContinue
        } catch { }
        if ($taskInfo) {
            Write-Output ("TASK_RESULT=" + $taskInfo.LastTaskResult)
            Write-Output ("TASK_LAST_RUN=" + $taskInfo.LastRunTime)
        }
    } elseif ($psexecDiagnostics) {
        Write-Output ("PSEXEC_ERROR=" + $psexecDiagnostics)
    }
    if (Test-Path $outPath) {
        Get-Content -Path $outPath | ForEach-Object { Write-Output $_ }
    }
    if (Test-Path $errPath) {
        Get-Content -Path $errPath | ForEach-Object { Write-Output $_ }
    }
    Cleanup
    exit 124
}

$exitCode = [int](Get-Content -Path $codePath -Raw)

if (Test-Path $outPath) {
    Get-Content -Path $outPath | ForEach-Object { Write-Output $_ }
}
if (Test-Path $errPath) {
    Get-Content -Path $errPath | ForEach-Object { Write-Output $_ }
}
if (Test-Path $timingPath) {
    Get-Content -Path $timingPath | ForEach-Object { Write-Output "TIMING: $_" }
}
if ($scheduleStart -and $waitStart -and $waitEnd) {
    $queueMs = [int](($waitStart - $scheduleStart).TotalMilliseconds)
    $waitMs = [int](($waitEnd - $waitStart).TotalMilliseconds)
    Write-Output "TIMING: schedule_ms=$queueMs; wait_ms=$waitMs"
}

if ($TimeoutPath -and (Test-Path $TimeoutPath)) {
    Remove-Item -Path $TimeoutPath -Force -ErrorAction SilentlyContinue
}

Cleanup
exit $exitCode
