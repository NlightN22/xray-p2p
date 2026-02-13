from __future__ import annotations

import time

import pytest

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env

CLIENT_TUN = "xp2pc"
SERVER_TUN = "xp2ps"
CLIENT_ADDR = "198.18.0.1/30"
SERVER_ADDR = "198.18.0.5/30"
SERVICE_TIMEOUT = 45.0
POLL_INTERVAL = 1.5

pytestmark = [pytest.mark.host, pytest.mark.linux]


def _xp2p(host, *args: str, check: bool = False):
    result = openwrt_env.run_xp2p(host, *args)
    if check and result.rc != 0:
        pytest.fail(
            f"xp2p command failed (exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return result


def _wait_for_service_state(host, role: str, expected_active: bool) -> None:
    deadline = time.time() + SERVICE_TIMEOUT
    script = f"/etc/init.d/xp2p-{role}"
    last = None
    while time.time() < deadline:
        result = host.run(f"{script} running")
        active = result.rc == 0
        if active == expected_active:
            return
        last = result
        time.sleep(POLL_INTERVAL)
    stdout = getattr(last, "stdout", "") or ""
    stderr = getattr(last, "stderr", "") or ""
    state = "active" if expected_active else "inactive"
    raise AssertionError(
        f"xp2p {role} service did not reach {state} state.\nSTDOUT:\n{stdout}\nSTDERR:\n{stderr}"
    )


def _assert_tun_addr(host, name: str, addr: str, timeout_seconds: int | None = None) -> None:
    args = [name, addr]
    if timeout_seconds is not None:
        args.append(str(timeout_seconds))
    result = openwrt_env.run_guest_script(
        host,
        "scripts/openwrt/assert_tun_addr.sh",
        *args,
    )
    assert result.rc == 0, (
        "TUN address check failed.\n"
        f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
    )


@pytest.mark.host
@pytest.mark.linux
def test_openwrt_client_service_brings_up_tun(openwrt_host, xp2p_openwrt_ipk):
    openwrt_env.install_ipk_on_host(openwrt_host, xp2p_openwrt_ipk, force=True)
    runner = lambda *cmd, check=False: _xp2p(openwrt_host, *cmd, check=check)
    helpers.cleanup_client_install(openwrt_host, runner)
    try:
        runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.80.10",
            "--user",
            "tun-runtime-client@example.com",
            "--password",
            "tun-runtime-client-pass",
            "--force",
            check=True,
        )
        runner("client", "service", "start", check=True)
        _wait_for_service_state(openwrt_host, "client", expected_active=True)
        _assert_tun_addr(openwrt_host, CLIENT_TUN, CLIENT_ADDR)
    finally:
        runner("client", "service", "stop")
        helpers.cleanup_client_install(openwrt_host, runner)


@pytest.mark.host
@pytest.mark.linux
def test_openwrt_server_service_brings_up_tun(openwrt_host, xp2p_openwrt_ipk):
    openwrt_env.install_ipk_on_host(openwrt_host, xp2p_openwrt_ipk, force=True)
    runner = lambda *cmd, check=False: _xp2p(openwrt_host, *cmd, check=check)
    helpers.cleanup_server_install(openwrt_host, runner)
    try:
        runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--port",
            "62150",
            "--host",
            "tun-runtime-server.example.com",
            "--force",
            check=True,
        )
        runner("server", "service", "start", check=True)
        _wait_for_service_state(openwrt_host, "server", expected_active=True)
        _assert_tun_addr(openwrt_host, SERVER_TUN, SERVER_ADDR, timeout_seconds=30)
    finally:
        runner("server", "service", "stop")
        helpers.cleanup_server_install(openwrt_host, runner)
