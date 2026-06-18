from __future__ import annotations

import time
from pathlib import PurePosixPath

from testinfra.host import Host

from tests.host.linux import _helpers as helpers
from tests.host.linux import env as linux_env

WRAPPER_LOG = PurePosixPath("/tmp/xp2p-loop-spike-xray.log")
RUN_LOG = PurePosixPath("/tmp/xp2p-client-run.log")
XRAY_BACKUP = PurePosixPath("/tmp/xp2p-real-xray-backup")


def install_synthetic_xray(host: Host, *, fd_count: int = 1200) -> None:
    result = linux_env.run_guest_script(
        host,
        "scripts/linux/install_loop_spike_xray.sh",
        helpers.XRAY_BINARY.as_posix(),
        WRAPPER_LOG.as_posix(),
        str(fd_count),
    )
    if result.rc != 0:
        raise AssertionError(
            "Failed to install synthetic xray.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def backup_xray(host: Host) -> None:
    result = host.run(
        f"sudo -n /bin/sh -c 'test -x \"$1\" && cp -fL \"$1\" \"$2\" && chmod 0755 \"$2\"' "
        f"-- {helpers.XRAY_BINARY.as_posix()} {XRAY_BACKUP.as_posix()}"
    )
    if result.rc != 0:
        raise AssertionError(f"Failed to backup xray.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}")


def restore_xray(host: Host) -> None:
    host.run(
        f"sudo -n /bin/sh -c 'if [ -f \"$2\" ]; then "
        f"rm -f \"$1\" \"$1\".fd* \"$1\".log \"$1\".fd*.log \"$1\".loop-spike-tmp; "
        f"cp -f \"$2\" \"$1\"; chmod 0755 \"$1\"; rm -f \"$2\"; fi' "
        f"-- {helpers.XRAY_BINARY.as_posix()} {XRAY_BACKUP.as_posix()}"
    )


def wait_for_quarantine(host: Host, *, timeout_seconds: float = 45.0) -> dict:
    deadline = time.time() + timeout_seconds
    last_state: dict | None = None
    while time.time() < deadline:
        if linux_env.path_exists(host, helpers.CLIENT_APPLIED_STATE_FILE):
            last_state = helpers.read_client_applied_state(host)
            runtime = last_state.get("runtime") or {}
            if runtime.get("status") == "quarantined":
                return last_state
        time.sleep(1.0)
    raise AssertionError(f"Client runtime was not quarantined. Last state: {last_state}")


def assert_runtime_quarantine(state: dict) -> None:
    runtime = state.get("runtime") or {}
    loop = runtime.get("loop_protection") or {}
    assert runtime.get("status") == "quarantined"
    assert runtime.get("reason") == "fd_socket_spike"
    assert int(loop.get("fd_before") or 0) > 0
    assert int(loop.get("fd_after") or 0) >= 1024
    assert int(loop.get("fd_delta") or 0) >= 512
    assert loop.get("action") == "kill_xray"


def assert_xray_stopped(host: Host) -> None:
    result = host.run("pgrep -af '/etc/xp2p/bin/[x]ray' || true")
    assert (result.stdout or "").strip() == "", f"synthetic xray still running:\n{result.stdout}"


def assert_client_state_output(output: str) -> None:
    expected = (
        "RUNTIME_STATUS",
        "STATUS=quarantined",
        "REASON=fd_socket_spike",
        "FD_BEFORE=",
        "FD_AFTER=",
        "FD_DELTA=",
        "ACTION=kill_xray",
    )
    for marker in expected:
        assert marker in output, f"client state output missing {marker!r}:\n{output}"


def assert_quarantine_delay_logged(host: Host) -> None:
    log = linux_env.read_text(host, RUN_LOG) if linux_env.path_exists(host, RUN_LOG) else ""
    assert "client loop protection quarantine delay" in log, log


def dump_loop_failure(host: Host, label: str) -> None:
    helpers.dump_failure_state(host, label)
    for path in (RUN_LOG, WRAPPER_LOG):
        result = host.run(f"sudo -n tail -n 200 {path.as_posix()} 2>/dev/null || true")
        if result.stdout:
            print(f"--- {path} ---")
            print(result.stdout)
