from contextlib import contextmanager
from pathlib import Path
import time

import pytest
from testinfra.host import Host

from . import env as _env

SERVER_RUN_STABILIZE_SECONDS = 15


def _start_xp2p_server_run(
    host: Host,
    install_dir: str,
    config_dir: str,
    log_relative: str,
    *,
    allow_mismatch: bool = False,
    output_log_path: str | None = None,
) -> int:
    rel = Path(log_relative).as_posix()
    if rel.lower().startswith("logs/"):
        rel = rel[5:]
    log_abs = str(_env.LOGS_DIR / rel)
    if output_log_path is None:
        output_log_path = str(_env.LOGS_DIR / "xp2p-server-run.out")
    parameters: dict[str, object] = {
        "Xp2pPath": str(_env.XP2P_EXE),
        "InstallDir": install_dir,
        "ConfigDir": config_dir,
        "LogRelative": log_relative,
        "LogPath": log_abs,
        "StabilizeSeconds": str(SERVER_RUN_STABILIZE_SECONDS),
    }
    if allow_mismatch:
        parameters["AllowMismatch"] = "1"
    parameters["OutputLogPath"] = output_log_path

    def _read_log(path: str) -> str:
        if _env.path_exists(host, path):
            return _env.read_text(host, path)
        return "<missing>"

    def _format_logs() -> str:
        xp2p_log = _read_log(output_log_path)
        xray_log = _read_log(log_abs)
        return (
            f"xp2p run log ({output_log_path}):\n{xp2p_log}\n"
            f"xray log ({log_abs}):\n{xray_log}"
        )

    def _should_retry(xp2p_log: str) -> bool:
        if "New-NetIPAddress : Element not found" in xp2p_log:
            return True
        if "Failed to register rings" in xp2p_log:
            return True
        return False

    last_result: CommandResult | None = None
    for attempt in range(2):
        result = _env.run_guest_script(
            host,
            "scripts/start_xp2p_server_run.ps1",
            **parameters,
        )
        last_result = result
        stdout = (result.stdout or "").strip()

        if result.rc == 0:
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

        if "__XP2P_MISSING__" in stdout:
            pytest.skip(
                f"xp2p.exe not found on {_env.DEFAULT_SERVER} at {_env.XP2P_EXE}. "
                "Provision the guest before running host tests."
            )
        if "__XP2P_CREATE_FAIL__" in stdout:
            pytest.fail(
                "Failed to spawn xp2p server run via Win32_Process.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}\n{_format_logs()}"
            )
        if "__XP2P_EXIT__" in stdout:
            xp2p_log = _read_log(output_log_path)
            if attempt == 0 and _should_retry(xp2p_log):
                if "Failed to register rings" in xp2p_log:
                    _env.remove_tun_adapters(host, ["xp2pc", "xp2ps", "Xray Tunnel"])
                _env.run_guest_script(host, "scripts/kill_xp2p_processes.ps1")
                time.sleep(2)
                continue
            pytest.fail(
                "xp2p server run exited before stabilization period elapsed.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}\n{_format_logs()}"
            )
        if "__XP2P_TIMEOUT__" in stdout:
            pytest.fail(
                "xp2p server run did not start xray-core in time.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}\n{_format_logs()}"
            )
        pytest.fail(
            "Failed to start xp2p server run on "
            f"{_env.DEFAULT_SERVER}.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}\n{_format_logs()}"
        )

    assert last_result is not None
    pytest.fail(
        "Failed to start xp2p server run on "
        f"{_env.DEFAULT_SERVER}.\nSTDOUT:\n{last_result.stdout}\nSTDERR:\n{last_result.stderr}\n{_format_logs()}"
    )


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
        rel = Path(log_relative).as_posix()
        if rel.lower().startswith("logs/"):
            rel = rel[5:]
        log_file = str(_env.LOGS_DIR / rel)
        yield {"pid": pid_value, "log_path": log_file}
    finally:
        if pid_value is not None:
            _stop_process(host, pid_value)


@contextmanager
def xp2p_server_run_session_with_env(
    host: Host,
    install_dir: str,
    config_dir: str,
    log_relative: str,
    *,
    allow_mismatch: bool = False,
    output_log_path: str | None = None,
):
    pid_value = None
    try:
        pid_value = _start_xp2p_server_run(
            host,
            install_dir,
            config_dir,
            log_relative,
            allow_mismatch=allow_mismatch,
            output_log_path=output_log_path,
        )
        rel = Path(log_relative).as_posix()
        if rel.lower().startswith("logs/"):
            rel = rel[5:]
        log_file = str(_env.LOGS_DIR / rel)
        yield {"pid": pid_value, "log_path": log_file}
    finally:
        if pid_value is not None:
            _stop_process(host, pid_value)
