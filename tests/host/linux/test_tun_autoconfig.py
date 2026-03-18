from __future__ import annotations

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import env as linux_env

CLIENT_TUN = "xp2pc"
SERVER_TUN = "xp2ps"


def _networkd_path(name: str) -> str:
    return f"/etc/systemd/network/90-{name}.network"


@pytest.mark.host
@pytest.mark.linux
def test_linux_tun_autoconfig_client_no_networkd(client_host, xp2p_client_runner):
    try:
        result = linux_env.run_xp2p_with_env(
            client_host,
            {"XP2P_CLIENT_TUN_ENABLED": "false"},
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.0.10",
            "--user",
            "tun-client@example.com",
            "--password",
            "tun-client-pass",
            "--force",
        )
        if result.rc != 0:
            pytest.fail(
                "xp2p client install failed.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )

        assert not helpers.path_exists(client_host, _networkd_path(CLIENT_TUN)), (
            "Networkd file should not be created for client install"
        )
    finally:
        pass


@pytest.mark.host
@pytest.mark.linux
def test_linux_tun_autoconfig_server_no_networkd(server_host, xp2p_server_runner):
    try:
        result = linux_env.run_xp2p_with_env(
            server_host,
            {"XP2P_SERVER_TUN_ENABLED": "false"},
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--port",
            "62022",
            "--host",
            "tun-server.example",
            "--force",
        )
        if result.rc != 0:
            pytest.fail(
                "xp2p server install failed.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )

        assert not helpers.path_exists(server_host, _networkd_path(SERVER_TUN)), (
            "Networkd file should not be created for server install"
        )
    finally:
        pass
