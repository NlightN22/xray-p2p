from __future__ import annotations

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import env as linux_env

CLIENT_TUN = "xp2pc"
SERVER_TUN = "xp2ps"
CLIENT_ADDR = "198.18.0.1/30"
SERVER_ADDR = "198.18.0.5/30"


def _start_service(role: str, runner, host) -> None:
    host.run("sudo -n systemctl daemon-reload >/dev/null 2>&1 || true")
    runner(role, "service", "start", check=True)


def _stop_service(role: str, runner) -> None:
    runner(role, "service", "stop")


def _assert_tun_addr(host, name: str, addr: str) -> None:
    result = linux_env.run_guest_script(
        host,
        "scripts/linux/assert_tun_addr.sh",
        name,
        addr,
    )
    assert result.rc == 0, (
        "TUN address check failed.\n"
        f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
    )


@pytest.mark.host
@pytest.mark.linux
def test_client_service_brings_up_tun(client_host, xp2p_client_runner):
    helpers.cleanup_client_install(client_host, xp2p_client_runner)
    try:
        xp2p_client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.10.80",
            "--user",
            "tun-runtime-client@example.com",
            "--password",
            "tun-runtime-client-pass",
            "--force",
            check=True,
        )
        _start_service("client", xp2p_client_runner, client_host)
        _assert_tun_addr(client_host, CLIENT_TUN, CLIENT_ADDR)
    finally:
        _stop_service("client", xp2p_client_runner)
        helpers.cleanup_client_install(client_host, xp2p_client_runner)


@pytest.mark.host
@pytest.mark.linux
def test_server_service_brings_up_tun(server_host, xp2p_server_runner):
    helpers.cleanup_server_install(server_host, xp2p_server_runner)
    try:
        xp2p_server_runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--port",
            "62140",
            "--host",
            "tun-runtime-server.example.com",
            "--force",
            check=True,
        )
        _start_service("server", xp2p_server_runner, server_host)
        _assert_tun_addr(server_host, SERVER_TUN, SERVER_ADDR)
    finally:
        _stop_service("server", xp2p_server_runner)
        helpers.cleanup_server_install(server_host, xp2p_server_runner)
