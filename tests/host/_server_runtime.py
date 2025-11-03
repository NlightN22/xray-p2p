from contextlib import contextmanager
from pathlib import Path

import pytest
from testinfra.host import Host

from . import _env

SERVER_RUN_STABILIZE_SECONDS = 6


def _start_xp2p_server_run(host: Host, install_dir: str, config_dir: str, log_relative: str) -> int:
    log_abs = str(Path(install_dir) / Path(log_relative))
    result = _env.run_guest_script(
        host,
        "scripts/start_xp2p_server_run.ps1",
        Xp2pPath=str(_env.XP2P_EXE),
        InstallDir=install_dir,
        ConfigDir=config_dir,
        LogRelative=log_relative,
        LogPath=log_abs,
        StabilizeSeconds=str(SERVER_RUN_STABILIZE_SECONDS),
    )
    stdout = (result.stdout or "").strip()

    if result.rc != 0:
        if "__XP2P_MISSING__" in stdout:
            pytest.skip(
                f"xp2p.exe not found on {_env.DEFAULT_SERVER} at {_env.XP2P_EXE}. "
                "Provision the guest before running host tests."
            )
        if "__XP2P_CREATE_FAIL__" in stdout:
            pytest.fail(
                "Failed to spawn xp2p server run via Win32_Process.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        if "__XP2P_EXIT__" in stdout:
            pytest.fail(
                "xp2p server run exited before stabilization period elapsed.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        if "__XP2P_TIMEOUT__" in stdout:
            pytest.fail(
                "xp2p server run did not start xray-core in time.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        pytest.fail(
            "Failed to start xp2p server run on "
            f"{_env.DEFAULT_SERVER}.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )

    pid_value: int | None = None
    for line in stdout.splitlines():
        if line.startswith("PID="):
            pid_value = int(line.split("=", 1)[1])
            break
    if pid_value is None:
        pytest.fail(
            "Unexpected xp2p server run startup output:\n"
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
def xp2p_server_run_session(host: Host, install_dir: str, config_dir: str, log_relative: str):
    pid_value = None
    try:
        pid_value = _start_xp2p_server_run(host, install_dir, config_dir, log_relative)
        log_file = str(Path(install_dir) / Path(log_relative))
        yield {"pid": pid_value, "log_path": log_file}
    finally:
        if pid_value is not None:
            _stop_process(host, pid_value)
