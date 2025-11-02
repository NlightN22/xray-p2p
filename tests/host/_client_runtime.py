from contextlib import contextmanager
from pathlib import Path

import pytest
from testinfra.host import Host

from . import _env

CLIENT_RUN_STABILIZE_SECONDS = 6


def _escape(path: str) -> str:
    return path.replace("\\", "\\\\")


def _start_xp2p_client_run(host: Host, install_dir: str, config_dir: str, log_relative: str) -> int:
    install_ps = _escape(install_dir)
    config_ps = _escape(config_dir)
    log_relative_ps = _escape(log_relative)
    log_abs = str(Path(install_dir) / Path(log_relative))
    log_abs_ps = _escape(log_abs)
    script = f"""
$ErrorActionPreference = 'Stop'
$xp2p = '{_env.XP2P_EXE_PS}'
$installDir = '{install_ps}'
$configDir = '{config_ps}'
$logRelative = '{log_relative_ps}'
$logPath = '{log_abs_ps}'
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

if (Test-Path $logPath) {{
    Remove-Item $logPath -Force -ErrorAction SilentlyContinue
}}

$commandLine = "`"$xp2p`" client run --quiet --path `"$installDir`" --config-dir `"$configDir`" --xray-log-file `"$logRelative`""
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


def _stop_process(host: Host, pid_value: int, install_dir: str, log_relative: str) -> None:
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
def xp2p_client_run_session(host: Host, install_dir: str, config_dir: str, log_relative: str):
    pid_value = None
    try:
        pid_value = _start_xp2p_client_run(host, install_dir, config_dir, log_relative)
        log_file = str(Path(install_dir) / Path(log_relative))
        yield {"pid": pid_value, "log_path": log_file}
    finally:
        if pid_value is not None:
            _stop_process(host, pid_value, install_dir, log_relative)
