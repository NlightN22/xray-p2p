from __future__ import annotations

from pathlib import PurePosixPath

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import env as linux_env

CLIENT_OUTBOUNDS = helpers.CLIENT_CONFIG_DIR / "outbounds.json"
CLIENT_LOG_PATH = helpers.CLIENT_LOG_FILE


def _assert_outbound(data: dict, address: str, password: str, email: str, server_name: str) -> None:
    primary = data["outbounds"][0]["settings"]["servers"][0]
    assert primary["address"] == address
    assert primary["password"] == password
    assert primary["email"] == email
    tls_settings = data["outbounds"][0]["streamSettings"]["tlsSettings"]
    assert tls_settings["serverName"] == server_name


def _cleanup(client_host, xp2p_client_runner) -> None:
    helpers.cleanup_client_install(client_host, xp2p_client_runner)


@pytest.mark.host
@pytest.mark.linux
def test_client_install_and_force_overwrites(client_host, xp2p_client_runner):
    _cleanup(client_host, xp2p_client_runner)
    try:
        xp2p_client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--server-address",
            "10.55.0.10",
            "--user",
            "alpha@example.com",
            "--password",
            "test_password123",
            "--force",
            check=True,
        )

        data = helpers.read_json(client_host, CLIENT_OUTBOUNDS)
        _assert_outbound(data, "10.55.0.10", "test_password123", "alpha@example.com", "10.55.0.10")

        xp2p_client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--server-address",
            "10.55.0.11",
            "--user",
            "beta@example.com",
            "--password",
            "override_password456",
            "--server-name",
            "vpn.example.local",
            "--force",
            check=True,
        )

        updated = helpers.read_json(client_host, CLIENT_OUTBOUNDS)
        _assert_outbound(updated, "10.55.0.11", "override_password456", "beta@example.com", "vpn.example.local")
    finally:
        _cleanup(client_host, xp2p_client_runner)


@pytest.mark.host
@pytest.mark.linux
def test_client_install_from_link(client_host, xp2p_client_runner):
    _cleanup(client_host, xp2p_client_runner)
    try:
        link = (
            "trojan://linkpass@link.example.test:62022?"
            "allowInsecure=1&security=tls&sni=link.example.test#link@example.com"
        )
        xp2p_client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--link",
            link,
            "--force",
            check=True,
        )
        data = helpers.read_json(client_host, CLIENT_OUTBOUNDS)
        _assert_outbound(data, "link.example.test", "linkpass", "link@example.com", "link.example.test")
        tls_settings = data["outbounds"][0]["streamSettings"]["tlsSettings"]
        assert tls_settings.get("allowInsecure") is True
    finally:
        _cleanup(client_host, xp2p_client_runner)


@pytest.mark.host
@pytest.mark.linux
def test_client_install_requires_force_when_state_exists(client_host, xp2p_client_runner):
    _cleanup(client_host, xp2p_client_runner)
    try:
        xp2p_client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--server-address",
            "10.55.0.20",
            "--user",
            "state@example.com",
            "--password",
            "state-pass",
            "--force",
            check=True,
        )

        result = xp2p_client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--server-address",
            "10.55.0.21",
            "--user",
            "state2@example.com",
            "--password",
            "state-pass-2",
            check=False,
        )
        assert result.rc != 0, "Expected install to fail when state marker exists and --force not supplied"
        combined = f"{result.stdout}\n{result.stderr}".lower()
        assert "client already installed" in combined
    finally:
        _cleanup(client_host, xp2p_client_runner)


@pytest.mark.host
@pytest.mark.linux
def test_client_install_recovers_without_state_marker(client_host, xp2p_client_runner):
    _cleanup(client_host, xp2p_client_runner)
    try:
        xp2p_client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--server-address",
            "10.55.0.30",
            "--user",
            "nostate@example.com",
            "--password",
            "nostate-pass",
            "--force",
            check=True,
        )

        for state_file in helpers.CLIENT_STATE_FILES:
            helpers.remove_path(client_host, state_file)
            assert not helpers.path_exists(client_host, state_file)

        xp2p_client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--server-address",
            "10.55.0.31",
            "--user",
            "nostate2@example.com",
            "--password",
            "nostate-pass-2",
            check=True,
        )

        assert any(helpers.path_exists(client_host, path) for path in helpers.CLIENT_STATE_FILES), (
            "Expected client install-state markers to be recreated"
        )
    finally:
        _cleanup(client_host, xp2p_client_runner)
