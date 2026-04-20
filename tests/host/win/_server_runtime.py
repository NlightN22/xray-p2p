from contextlib import contextmanager
import time

import pytest
from testinfra.host import Host

from . import env as _env
from .flows import apply as apply_flow

SERVER_RUN_STABILIZE_SECONDS = 15
APPLY_REQUEST_TIMEOUT = 90.0


def _wait_for_apply_request_clear(host: Host, timeout: float = APPLY_REQUEST_TIMEOUT) -> None:
    apply_flow.wait_for_apply_request_clear(host, timeout=timeout)


def _start_xp2p_server_run(
    host: Host,
    install_dir: str,
    config_dir: str,
    *,
    allow_mismatch: bool = False,
    output_log_path: str | None = None,
) -> int:
    if output_log_path is None:
        output_log_path = str(_env.LOGS_DIR / "xp2p-server-run.out")
    parameters: dict[str, object] = {
        "Xp2pPath": str(_env.XP2P_EXE),
        "InstallDir": install_dir,
        "ConfigDir": config_dir,
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
        return f"xp2p run log ({output_log_path}):\n{xp2p_log}"

    def _should_retry(xp2p_log: str) -> bool:
        if "New-NetIPAddress : Element not found" in xp2p_log:
            return True
        if "Failed to register rings" in xp2p_log:
            return True
        if "Failed to find matching adapter name" in xp2p_log:
            return True
        if "Only one usage of each socket address" in xp2p_log:
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
                _env.stop_xp2p_processes(host)
                time.sleep(2)
                continue
            pytest.fail(
                "xp2p server run exited before stabilization period elapsed.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}\n{_format_logs()}"
        )
        if "__XP2P_TIMEOUT__" in stdout:
            xp2p_log = _read_log(output_log_path)
            if attempt == 0 and _should_retry(xp2p_log):
                _env.remove_tun_adapters(host, ["xp2pc", "xp2ps", "Xray Tunnel"])
                _env.stop_xp2p_processes(host)
                time.sleep(2)
                continue
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
def xp2p_server_run_session(host: Host, install_dir: str, config_dir: str):
    pid_value = None
    try:
        pid_value = _start_xp2p_server_run(host, install_dir, config_dir)
        _wait_for_apply_request_clear(host)
        yield {"pid": pid_value}
    finally:
        if pid_value is not None:
            _stop_process(host, pid_value)


@contextmanager
def xp2p_server_run_session_with_env(
    host: Host,
    install_dir: str,
    config_dir: str,
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
            allow_mismatch=allow_mismatch,
            output_log_path=output_log_path,
            )
        _wait_for_apply_request_clear(host)
        yield {"pid": pid_value}
    finally:
        if pid_value is not None:
            _stop_process(host, pid_value)
