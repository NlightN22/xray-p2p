from contextlib import contextmanager
from pathlib import Path

import pytest
from testinfra.host import Host

from . import _env

CLIENT_LOG_FILE = Path(r"C:\Program Files\xp2p\logs\client.err")
CLIENT_LOG_FILE_PS = str(CLIENT_LOG_FILE).replace("\\", "\\\\")
CLIENT_RUN_LOG_ARG = "logs\\\\client.err"
CLIENT_RUN_STABILIZE_SECONDS = 6


def _start_xp2p_client_run(host: Host) -> int:
    script = f"""
$ErrorActionPreference = 'Stop'
$xp2p = '{_env.XP2P_EXE_PS}'
if (-not (Test-Path $xp2p)) {{
    Write-Output '__XP2P_MISSING__'
    exit 3
}}

$existing = Get-Process -Name xp2p -ErrorAction SilentlyContinue | Where-Object {{ $_.Path -eq $xp2p }}
if ($existing) {{
    foreach ($item in $existing) {{
        try {{
            Stop-Process -Id $item.Id -Force -ErrorAction SilentlyContinue
        }} catch {{ }}
    }}
    Start-Sleep -Seconds 1
}}

$logPath = '{CLIENT_LOG_FILE_PS}'
if (Test-Path $logPath) {{
    Remove-Item $logPath -Force -ErrorAction SilentlyContinue
}}

$commandLine = "`"$xp2p`" client run --quiet --xray-log-file {CLIENT_RUN_LOG_ARG}"
$workingDir = Split-Path $xp2p
$createResult = Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{{ CommandLine = $commandLine; CurrentDirectory = $workingDir }}
if ($createResult.ReturnValue -ne 0 -or -not $createResult.ProcessId) {{
    Write-Output ('__XP2P_CREATE_FAIL__' + $createResult.ReturnValue)
    exit 4
}}
$processId = [int]$createResult.ProcessId
$deadline = (Get-Date).AddSeconds({CLIENT_RUN_STABILIZE_SECONDS})

while ((Get-Date) -lt $deadline) {{
    $proc = Get-Process -Id $processId -ErrorAction SilentlyContinue
    if (-not $proc) {{
        Write-Output '__XP2P_EXIT__'
        exit 6
    }}
    Start-Sleep -Seconds 1
}}

Write-Output ('PID=' + $processId)
exit 0
"""
    result = _env.run_powershell(host, script)
    stdout = (result.stdout or "").strip()

    if result.rc != 0:
        if "__XP2P_MISSING__" in stdout:
            pytest.skip(
                f"xp2p.exe not found on {_env.DEFAULT_CLIENT} at {_env.XP2P_EXE}. "
                "Provision the guest before running host tests."
            )
        if "__XP2P_CREATE_FAIL__" in stdout:
            pytest.fail(
                "Failed to spawn xp2p client run via Win32_Process.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        if "__XP2P_EXIT__" in stdout:
            pytest.fail(
                "xp2p client run exited before stabilization period elapsed.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        pytest.fail(
            "Failed to start xp2p client run on "
            f"{_env.DEFAULT_CLIENT}.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )

    pid_value: int | None = None
    for line in stdout.splitlines():
        if line.startswith("PID="):
            pid_value = int(line.split("=", 1)[1])
            break
    if pid_value is None:
        pytest.fail(
            "Unexpected xp2p client run startup output:\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return pid_value


def _stop_process(host: Host, pid_value: int) -> None:
    script = f"""
$pidValue = {pid_value}
if ($pidValue -le 0) {{
    exit 0
}}
$proc = Get-Process -Id $pidValue -ErrorAction SilentlyContinue
if ($proc) {{
    try {{
        Stop-Process -Id $pidValue -Force -ErrorAction SilentlyContinue
    }} catch {{ }}
}}
Start-Sleep -Milliseconds 200
$xray = Get-Process -Name xray -ErrorAction SilentlyContinue
if ($xray) {{
    foreach ($item in $xray) {{
        try {{
            Stop-Process -Id $item.Id -Force -ErrorAction SilentlyContinue
        }} catch {{ }}
    }}
}}
exit 0
"""
    _env.run_powershell(host, script)


@contextmanager
def xp2p_client_run_session(host: Host):
    pid_value = None
    try:
        pid_value = _start_xp2p_client_run(host)
        yield {"pid": pid_value, "log_path": CLIENT_LOG_FILE}
    finally:
        if pid_value is not None:
            _stop_process(host, pid_value)
