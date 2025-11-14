from __future__ import annotations

from pathlib import PurePosixPath

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import env as linux_env

CLIENT_OUTBOUNDS = helpers.CLIENT_CONFIG_DIR / "outbounds.json"
CLIENT_LOG_PATH = helpers.CLIENT_LOG_FILE
CLIENT_ROUTING = helpers.CLIENT_CONFIG_DIR / "routing.json"
CLIENT_STATE_FILE = helpers.CLIENT_STATE_FILES[0]


def _expected_tag(host: str) -> str:
    cleaned = host.strip().lower()
    result = []
    last_dash = False
    for char in cleaned:
        if char.isalnum():
            result.append(char)
            last_dash = False
            continue
        if char == "-":
            result.append(char)
            last_dash = False
            continue
        if not last_dash:
            result.append("-")
            last_dash = True
    sanitized = "".join(result).strip("-")
    if not sanitized:
        sanitized = "endpoint"
    return f"proxy-{sanitized}"


def _find_outbound(data: dict, tag: str) -> dict:
    for outbound in data.get("outbounds", []):
        if outbound.get("tag") == tag:
            return outbound
    raise AssertionError(f"Expected outbound with tag {tag}")


def _assert_outbound(
    data: dict, host: str, password: str, email: str, server_name: str, allow_insecure: bool = False
) -> None:
    outbound = _find_outbound(data, _expected_tag(host))
    server = outbound["settings"]["servers"][0]
    assert server["address"] == host
    assert server["password"] == password
    assert server["email"] == email
    tls_settings = outbound["streamSettings"]["tlsSettings"]
    assert tls_settings["serverName"] == server_name
    assert bool(tls_settings.get("allowInsecure")) is bool(allow_insecure)


def _assert_routing_rule(data: dict, host: str) -> None:
    tag = _expected_tag(host)
    rules = data.get("routing", {}).get("rules", [])
    for rule in rules:
        if rule.get("outboundTag") == tag and host in rule.get("ip", []):
            return
    raise AssertionError(f"Expected routing rule for {host} -> {tag}")


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
            check=True,
        )

        updated = helpers.read_json(client_host, CLIENT_OUTBOUNDS)
        _assert_outbound(updated, "10.55.0.10", "test_password123", "alpha@example.com", "10.55.0.10")
        _assert_outbound(updated, "10.55.0.11", "override_password456", "beta@example.com", "vpn.example.local")

        routing = helpers.read_json(client_host, CLIENT_ROUTING)
        _assert_routing_rule(routing, "10.55.0.10")
        _assert_routing_rule(routing, "10.55.0.11")

        state = helpers.read_json(client_host, CLIENT_STATE_FILE)
        recorded_hosts = {entry["hostname"] for entry in state.get("endpoints", [])}
        assert recorded_hosts == {"10.55.0.10", "10.55.0.11"}

        duplicate = xp2p_client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--server-address",
            "10.55.0.10",
            "--user",
            "gamma@example.com",
            "--password",
            "newpass",
            check=False,
        )
        assert duplicate.rc != 0, "Expected duplicate endpoint install to fail without --force"
        assert "endpoint 10.55.0.10 already exists" in duplicate.stderr.lower() + duplicate.stdout.lower()

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
            "gamma@example.com",
            "--password",
            "forcepass",
            "--server-name",
            "override.linux",
            "--force",
            check=True,
        )

        refreshed = helpers.read_json(client_host, CLIENT_OUTBOUNDS)
        _assert_outbound(refreshed, "10.55.0.10", "forcepass", "gamma@example.com", "override.linux")
        _assert_outbound(refreshed, "10.55.0.11", "override_password456", "beta@example.com", "vpn.example.local")
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
        _assert_outbound(
            data, "link.example.test", "linkpass", "link@example.com", "link.example.test", allow_insecure=True
        )
    finally:
        _cleanup(client_host, xp2p_client_runner)


@pytest.mark.host
@pytest.mark.linux
def test_client_install_requires_force_for_duplicate_endpoint(client_host, xp2p_client_runner):
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
            "10.55.0.20",
            "--user",
            "state2@example.com",
            "--password",
            "state-pass-2",
            check=False,
        )
        assert result.rc != 0, "Expected duplicate endpoint install to fail without --force"
        combined = f"{result.stdout}\n{result.stderr}".lower()
        assert "endpoint 10.55.0.20 already exists" in combined

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
            "state2@example.com",
            "--password",
            "state-pass-2",
            "--force",
            check=True,
        )
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
