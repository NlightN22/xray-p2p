param(
    [Parameter(Mandatory = $true)]
    [string]$Xp2pPath,
    [Parameter(Mandatory = $true)]
    [string]$ArgsBase64
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

Import-Module ScheduledTasks -ErrorAction SilentlyContinue | Out-Null

try {
    $scheduleStart = Get-Date
    $action = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument "-NoProfile -ExecutionPolicy Bypass -File `"$scriptPath`""
    $trigger = New-ScheduledTaskTrigger -Once -At ((Get-Date).AddMinutes(5))
    Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger -RunLevel Highest -Force -User 'SYSTEM' | Out-Null
    Start-ScheduledTask -TaskName $taskName

    $waitStart = Get-Date
    $deadline = (Get-Date).AddMinutes(5)
    while ($true) {
        Start-Sleep -Seconds 1
        $state = (Get-ScheduledTask -TaskName $taskName).State
        if ($state -ne 'Running' -and $state -ne 'Queued') {
            break
        }
        if ((Get-Date) -gt $deadline) {
            Stop-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
            throw "xp2p scheduled task $taskName timed out."
        }
    }
    $waitEnd = Get-Date
} finally {
    Unregister-ScheduledTask -TaskName $taskName -Confirm:$false -ErrorAction SilentlyContinue | Out-Null
}

if (-not (Test-Path $codePath)) {
    Cleanup
    throw "xp2p scheduled task $taskName did not record exit code."
}

$exitCode = [int](Get-Content -Path $codePath -Raw)

if (Test-Path $outPath) {
    Get-Content -Path $outPath | ForEach-Object { Write-Output $_ }
}
if (Test-Path $errPath) {
    Get-Content -Path $errPath | ForEach-Object { Write-Error $_ }
}
if (Test-Path $timingPath) {
    Get-Content -Path $timingPath | ForEach-Object { Write-Output "TIMING: $_" }
}
if ($scheduleStart -and $waitStart -and $waitEnd) {
    $queueMs = [int](($waitStart - $scheduleStart).TotalMilliseconds)
    $waitMs = [int](($waitEnd - $waitStart).TotalMilliseconds)
    Write-Output "TIMING: schedule_ms=$queueMs; wait_ms=$waitMs"
}

Cleanup
exit $exitCode
