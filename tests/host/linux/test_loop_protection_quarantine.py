from __future__ import annotations

import time

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import _loop_protection as loop
from tests.host.linux import env as linux_env


@pytest.mark.host
@pytest.mark.linux
def test_client_run_quarantines_xray_on_fd_socket_spike(client_host, xp2p_client_runner):
    pid = ""
    try:
        xp2p_client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.0.10",
            "--user",
            "loop@example.com",
            "--password",
            "loop-password",
            "--mode",
            "proxy",
            check=True,
        )
        loop.backup_xray(client_host)
        loop.install_synthetic_xray(client_host)

        result = linux_env.run_guest_script(
            client_host,
            "scripts/linux/start_xp2p_run_with_env.sh",
            "client",
            helpers.INSTALL_ROOT.as_posix(),
            helpers.CLIENT_CONFIG_DIR_NAME,
            "1",
            "0",
        )
        assert result.rc == 0, f"failed to start client run:\n{result.stdout}\n{result.stderr}"
        pid_line = next((line for line in (result.stdout or "").splitlines() if line.startswith("__XP2P_PID__=")), "")
        assert pid_line, result.stdout
        pid = pid_line.split("=", 1)[1].strip()

        state = loop.wait_for_quarantine(client_host)
        loop.assert_runtime_quarantine(state)
        loop.assert_xray_stopped(client_host)

        state_result = xp2p_client_runner(
            "client",
            "state",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            check=True,
        )
        loop.assert_client_state_output(state_result.stdout or "")

        time.sleep(2.0)
        rerun_state = loop.wait_for_quarantine(client_host, timeout_seconds=5.0)
        loop.assert_runtime_quarantine(rerun_state)
        loop.assert_quarantine_delay_logged(client_host)
    except Exception:
        loop.dump_loop_failure(client_host, "loop-protection-quarantine")
        raise
    finally:
        if pid:
            client_host.run(f"sudo -n kill {pid} >/dev/null 2>&1 || true")
        loop.restore_xray(client_host)
