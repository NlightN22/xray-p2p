from __future__ import annotations

from pathlib import PurePosixPath

import pytest

from tests.host.linux import _helpers as helpers

NETWORKD_DIR = PurePosixPath("/etc/systemd/network")
CLIENT_TUN = "xp2pc"
SERVER_TUN = "xp2ps"
CLIENT_ADDR = "198.18.0.1/30"
SERVER_ADDR = "198.18.0.5/30"
CLIENT_TABLE = 20090
SERVER_TABLE = 20091


def _networkd_path(name: str) -> PurePosixPath:
    return NETWORKD_DIR / f"90-{name}.network"


def _has_line(content: str, expected: str) -> bool:
    for line in content.splitlines():
        if line.strip() == expected:
            return True
    return False


def _assert_networkd_config(host, name: str, addr: str, table: int) -> None:
    path = _networkd_path(name)
    assert helpers.path_exists(host, path), f"Expected networkd file {path} to exist"
    content = helpers.read_text(host, path)
    assert _has_line(content, "# xp2p-managed"), "Expected xp2p-managed marker in networkd file"
    assert _has_line(content, f"Name = {name}"), f"Expected Name = {name} in networkd file"
    assert _has_line(content, f"Address = {addr}"), f"Expected Address = {addr} in networkd file"
    assert _has_line(content, f"Table = {table}"), f"Expected Table = {table} in networkd file"
    assert _has_line(content, "Destination = 0.0.0.0/0"), "Expected default route destination in networkd file"
    assert "[RoutingPolicyRule]" not in content, "Unexpected RoutingPolicyRule stanza in networkd file"
    assert "From =" not in content, "Unexpected RoutingPolicyRule From in networkd file"


def _cleanup_networkd_if_managed(host, name: str) -> None:
    path = _networkd_path(name)
    if not helpers.path_exists(host, path):
        return
    content = helpers.read_text(host, path)
    if "xp2p-managed" in content:
        helpers.remove_path(host, path)


@pytest.mark.host
@pytest.mark.linux
def test_linux_tun_autoconfig_client_networkd(client_host, xp2p_client_runner):
    helpers.cleanup_client_install(client_host, xp2p_client_runner)
    _cleanup_networkd_if_managed(client_host, CLIENT_TUN)
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
            "networkd-client@example.com",
            "--password",
            "networkd-client-pass",
            "--force",
            check=True,
        )

        _assert_networkd_config(client_host, CLIENT_TUN, CLIENT_ADDR, CLIENT_TABLE)

        helpers.cleanup_client_install(client_host, xp2p_client_runner)
        assert not helpers.path_exists(client_host, _networkd_path(CLIENT_TUN)), (
            "Expected client networkd file to be removed after xp2p client remove"
        )
    finally:
        if helpers.path_exists(client_host, _networkd_path(CLIENT_TUN)):
            _cleanup_networkd_if_managed(client_host, CLIENT_TUN)
        helpers.cleanup_client_install(client_host, xp2p_client_runner)


@pytest.mark.host
@pytest.mark.linux
def test_linux_tun_autoconfig_server_networkd(server_host, xp2p_server_runner):
    helpers.cleanup_server_install(server_host, xp2p_server_runner)
    _cleanup_networkd_if_managed(server_host, SERVER_TUN)
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
            "networkd-server.example",
            "--force",
            check=True,
        )

        _assert_networkd_config(server_host, SERVER_TUN, SERVER_ADDR, SERVER_TABLE)

        helpers.cleanup_server_install(server_host, xp2p_server_runner)
        assert not helpers.path_exists(server_host, _networkd_path(SERVER_TUN)), (
            "Expected server networkd file to be removed after xp2p server remove"
        )
    finally:
        if helpers.path_exists(server_host, _networkd_path(SERVER_TUN)):
            _cleanup_networkd_if_managed(server_host, SERVER_TUN)
        helpers.cleanup_server_install(server_host, xp2p_server_runner)


@pytest.mark.host
@pytest.mark.linux
def test_linux_tun_autoconfig_preserves_unmanaged_networkd(client_host, xp2p_client_runner):
    helpers.cleanup_client_install(client_host, xp2p_client_runner)
    _cleanup_networkd_if_managed(client_host, CLIENT_TUN)
    manual_path = _networkd_path(CLIENT_TUN)
    manual_content = "\n".join(
        [
            "[Match]",
            f"Name = {CLIENT_TUN}",
            "",
            "[Network]",
            "Address = 10.20.0.1/30",
            "",
        ]
    )
    try:
        xp2p_client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.0.11",
            "--user",
            "networkd-manual@example.com",
            "--password",
            "networkd-manual-pass",
            "--force",
            check=True,
        )
        helpers.write_text(client_host, manual_path, manual_content)

        helpers.cleanup_client_install(client_host, xp2p_client_runner)

        assert helpers.path_exists(client_host, manual_path), (
            "Expected unmanaged networkd file to remain after xp2p client remove"
        )
        assert helpers.read_text(client_host, manual_path) == manual_content
    finally:
        helpers.remove_path(client_host, manual_path)
        helpers.cleanup_client_install(client_host, xp2p_client_runner)
