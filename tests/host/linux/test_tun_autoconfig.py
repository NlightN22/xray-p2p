from __future__ import annotations

import pytest

from tests.host.linux import _helpers as helpers

CLIENT_TUN = "xp2pc"
SERVER_TUN = "xp2ps"


def _networkd_path(name: str) -> str:
    return f"/etc/systemd/network/90-{name}.network"


@pytest.mark.host
@pytest.mark.linux
def test_linux_tun_autoconfig_client_no_networkd(client_host, xp2p_client_runner):
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
            "10.55.0.10",
            "--user",
            "tun-client@example.com",
            "--password",
            "tun-client-pass",
            "--force",
            check=True,
        )

        assert not helpers.path_exists(client_host, _networkd_path(CLIENT_TUN)), (
            "Networkd file should not be created for client install"
        )
    finally:
        helpers.cleanup_client_install(client_host, xp2p_client_runner)


@pytest.mark.host
@pytest.mark.linux
def test_linux_tun_autoconfig_server_no_networkd(server_host, xp2p_server_runner):
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
            "62022",
            "--host",
            "tun-server.example",
            "--force",
            check=True,
        )

        assert not helpers.path_exists(server_host, _networkd_path(SERVER_TUN)), (
            "Networkd file should not be created for server install"
        )
    finally:
        helpers.cleanup_server_install(server_host, xp2p_server_runner)
