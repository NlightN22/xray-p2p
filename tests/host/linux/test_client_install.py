from __future__ import annotations

from pathlib import PurePosixPath

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import env as linux_env

CLIENT_OUTBOUNDS = helpers.CLIENT_CONFIG_DIR / "outbounds.json"
CLIENT_LOG_PATH = helpers.CLIENT_LOG_FILE
CLIENT_ROUTING = helpers.CLIENT_CONFIG_DIR / "routing.json"
CLIENT_STATE_FILE = helpers.CLIENT_STATE_FILES[0]


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
        helpers.assert_outbound(
            data,
            "10.55.0.10",
            "test_password123",
            "alpha@example.com",
            "10.55.0.10",
            allow_insecure=True,
        )

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
        helpers.assert_outbound(
            updated,
            "10.55.0.10",
            "test_password123",
            "alpha@example.com",
            "10.55.0.10",
            allow_insecure=True,
        )
        helpers.assert_outbound(
            updated,
            "10.55.0.11",
            "override_password456",
            "beta@example.com",
            "vpn.example.local",
            allow_insecure=True,
        )

        routing = helpers.read_json(client_host, CLIENT_ROUTING)
        helpers.assert_routing_rule(routing, "10.55.0.10")
        helpers.assert_routing_rule(routing, "10.55.0.11")

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
        helpers.assert_outbound(
            refreshed,
            "10.55.0.10",
            "forcepass",
            "gamma@example.com",
            "override.linux",
            allow_insecure=True,
        )
        helpers.assert_outbound(
            refreshed,
            "10.55.0.11",
            "override_password456",
            "beta@example.com",
            "vpn.example.local",
            allow_insecure=True,
        )
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
        helpers.assert_outbound(
            data, "link.example.test", "linkpass", "link@example.com", "link.example.test", allow_insecure=True
        )
    finally:
        _cleanup(client_host, xp2p_client_runner)


def _assert_no_endpoint(host: str, data: dict):
    tag = helpers.expected_proxy_tag(host)
    for outbound in data.get("outbounds", []):
        if outbound.get("tag") == tag:
            pytest.fail(f"Unexpected outbound {tag} still present")


@pytest.mark.host
@pytest.mark.linux
def test_client_remove_endpoint_and_list(client_host, xp2p_client_runner):
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
            "10.66.0.10",
            "--user",
            "delta@example.com",
            "--password",
            "delta-pass",
            check=True,
        )
        xp2p_client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--server-address",
            "10.66.0.11",
            "--user",
            "echo@example.com",
            "--password",
            "echo-pass",
            check=True,
        )

        list_result = xp2p_client_runner(
            "client",
            "list",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            check=True,
        ).stdout or ""
        assert "HOSTNAME" in list_result
        assert "10.66.0.10" in list_result
        assert "10.66.0.11" in list_result

        redirect_cidr = "10.200.0.0/16"
        host_tag = helpers.expected_proxy_tag("10.66.0.10")
        xp2p_client_runner(
            "client",
            "redirect",
            "add",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--cidr",
            redirect_cidr,
            "--tag",
            host_tag,
            check=True,
        )
        redirect_list = xp2p_client_runner(
            "client",
            "redirect",
            "list",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            check=True,
        ).stdout or ""
        assert redirect_cidr in redirect_list

        xp2p_client_runner(
            "client",
            "remove",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "10.66.0.10",
            check=True,
        )

        outbounds = helpers.read_json(client_host, CLIENT_OUTBOUNDS)
        helpers.assert_outbound(
            outbounds,
            "10.66.0.11",
            "echo-pass",
            "echo@example.com",
            "10.66.0.11",
            allow_insecure=True,
        )
        _assert_no_endpoint("10.66.0.10", outbounds)

        routing = helpers.read_json(client_host, CLIENT_ROUTING)
        helpers.assert_routing_rule(routing, "10.66.0.11")

        state = helpers.read_json(client_host, CLIENT_STATE_FILE)
        hosts = {entry.get("hostname") for entry in state.get("endpoints", [])}
        assert hosts == {"10.66.0.11"}

        redirect_list_after = xp2p_client_runner(
            "client",
            "redirect",
            "list",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            check=True,
        ).stdout or ""
        assert redirect_cidr not in redirect_list_after

        list_after = xp2p_client_runner(
            "client",
            "list",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            check=True,
        ).stdout or ""
        assert "10.66.0.11" in list_after
        assert "10.66.0.10" not in list_after

        xp2p_client_runner(
            "client",
            "remove",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--all",
            check=True,
        )

        final_list = xp2p_client_runner(
            "client",
            "list",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            check=True,
        ).stdout or ""
        assert "No client endpoints configured." in final_list
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
