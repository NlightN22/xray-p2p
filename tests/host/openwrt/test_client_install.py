from __future__ import annotations

import pytest

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env
from tests.host.tunnel import common as tunnel_common

CLIENT_OUTBOUNDS = helpers.CLIENT_CONFIG_DIR / "outbounds.json"
CLIENT_ROUTING = helpers.CLIENT_CONFIG_DIR / "routing.json"
CLIENT_STATE_FILE = helpers.CLIENT_STATE_FILES[0]

pytestmark = [pytest.mark.host, pytest.mark.linux]


def _runner(host):
    def _run(*args: str, check: bool = False):
        result = openwrt_env.run_xp2p(host, *args)
        if check and result.rc != 0:
            pytest.fail(
                "xp2p command failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        return result

    return _run


def _prepare_host(openwrt_host, xp2p_openwrt_ipk):
    openwrt_env.sync_build_output(openwrt_env.DEFAULT_OPENWRT_MACHINE)
    openwrt_env.install_ipk_on_host(openwrt_host, xp2p_openwrt_ipk)
    runner = _runner(openwrt_host)
    helpers.cleanup_client_install(openwrt_host, runner)
    helpers.remove_path(openwrt_host, helpers.HEARTBEAT_STATE_FILE)
    return runner


@pytest.mark.host
@pytest.mark.linux
def test_client_install_and_force_overwrites(openwrt_host, xp2p_openwrt_ipk):
    runner = _prepare_host(openwrt_host, xp2p_openwrt_ipk)
    try:
        runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.0.10",
            "--user",
            "alpha@example.com",
            "--password",
            "test_password123",
            check=True,
        )

        data = helpers.read_json(openwrt_host, CLIENT_OUTBOUNDS)
        helpers.assert_outbound(
            data,
            "10.55.0.10",
            "test_password123",
            "alpha@example.com",
            "10.55.0.10",
        )

        runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.0.11",
            "--user",
            "beta@example.com",
            "--password",
            "override_password456",
            "--sni",
            "vpn.example.local",
            check=True,
        )

        updated = helpers.read_json(openwrt_host, CLIENT_OUTBOUNDS)
        helpers.assert_outbound(
            updated,
            "10.55.0.10",
            "test_password123",
            "alpha@example.com",
            "10.55.0.10",
            allow_insecure=False,
        )
        helpers.assert_outbound(
            updated,
            "10.55.0.11",
            "override_password456",
            "beta@example.com",
            "vpn.example.local",
            allow_insecure=False,
        )

        routing = helpers.read_json(openwrt_host, CLIENT_ROUTING)
        helpers.assert_routing_rule(routing, "10.55.0.10")
        helpers.assert_routing_rule(routing, "10.55.0.11")

        state = helpers.read_json(openwrt_host, CLIENT_STATE_FILE)
        recorded_hosts = {entry["hostname"] for entry in state.get("endpoints", [])}
        assert recorded_hosts == {"10.55.0.10", "10.55.0.11"}

        duplicate = runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.0.10",
            "--user",
            "gamma@example.com",
            "--password",
            "newpass",
            check=False,
        )
        assert duplicate.rc != 0, "Expected duplicate endpoint install to fail without --force"
        combined = (duplicate.stderr or "") + (duplicate.stdout or "")
        combined = combined.lower()
        assert "endpoint 10.55.0.10" in combined and "already exists" in combined

        runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.0.10",
            "--user",
            "gamma@example.com",
            "--password",
            "forcepass",
            "--sni",
            "override.linux",
            "--force",
            check=True,
        )

        refreshed = helpers.read_json(openwrt_host, CLIENT_OUTBOUNDS)
        helpers.assert_outbound(
            refreshed,
            "10.55.0.10",
            "forcepass",
            "gamma@example.com",
            "override.linux",
            allow_insecure=False,
        )
        helpers.assert_outbound(
            refreshed,
            "10.55.0.11",
            "override_password456",
            "beta@example.com",
            "vpn.example.local",
            allow_insecure=False,
        )
    finally:
        helpers.cleanup_client_install(openwrt_host, runner)
        helpers.remove_path(openwrt_host, helpers.HEARTBEAT_STATE_FILE)


@pytest.mark.host
@pytest.mark.linux
def test_client_install_from_link(openwrt_host, xp2p_openwrt_ipk):
    runner = _prepare_host(openwrt_host, xp2p_openwrt_ipk)
    try:
        link = (
            "trojan://linkpass@link.example.test:62022?"
            "allowInsecure=1&security=tls&sni=link.example.test#link@example.com"
        )
        runner(
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
        data = helpers.read_json(openwrt_host, CLIENT_OUTBOUNDS)
        helpers.assert_outbound(
            data, "link.example.test", "linkpass", "link@example.com", "link.example.test", allow_insecure=True
        )
    finally:
        helpers.cleanup_client_install(openwrt_host, runner)
        helpers.remove_path(openwrt_host, helpers.HEARTBEAT_STATE_FILE)


@pytest.mark.host
@pytest.mark.linux
def test_client_install_from_link_without_allow_insecure(openwrt_host, xp2p_openwrt_ipk):
    runner = _prepare_host(openwrt_host, xp2p_openwrt_ipk)
    try:
        link = (
            "trojan://linkpass@link.example.test:62022?"
            "security=tls&sni=link.example.test#link@example.com"
        )
        runner(
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
        data = helpers.read_json(openwrt_host, CLIENT_OUTBOUNDS)
        helpers.assert_outbound(
            data, "link.example.test", "linkpass", "link@example.com", "link.example.test", allow_insecure=False
        )
    finally:
        helpers.cleanup_client_install(openwrt_host, runner)
        helpers.remove_path(openwrt_host, helpers.HEARTBEAT_STATE_FILE)


@pytest.mark.host
@pytest.mark.linux
def test_client_state_reports_multiple_endpoints(openwrt_host, xp2p_openwrt_ipk):
    runner = _prepare_host(openwrt_host, xp2p_openwrt_ipk)
    try:
        runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.0.40",
            "--user",
            "state-one@example.com",
            "--password",
            "state-pass-one",
            check=True,
        )
        link = (
            "trojan://statepass@link.example.test:62022?"
            "allowInsecure=1&security=tls&sni=link.example.test#state-two@example.com"
        )
        runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--link",
            link,
            check=True,
        )

        if helpers.path_exists(openwrt_host, helpers.HEARTBEAT_STATE_FILE):
            helpers.remove_path(openwrt_host, helpers.HEARTBEAT_STATE_FILE)

        result = runner(
            "client",
            "state",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            check=True,
        )
        rows = tunnel_common.parse_state_rows(result.stdout or "")
        expected_tags = {
            helpers.expected_proxy_tag("10.55.0.40"),
            helpers.expected_proxy_tag("link.example.test"),
        }
        expected_hosts = {"10.55.0.40", "link.example.test"}
        assert len(rows) == 2
        assert {row["TAG"] for row in rows} == expected_tags
        assert {row["HOST"] for row in rows} == expected_hosts
    finally:
        helpers.cleanup_client_install(openwrt_host, runner)
        helpers.remove_path(openwrt_host, helpers.HEARTBEAT_STATE_FILE)


def _assert_no_endpoint(host: str, data: dict):
    tag = helpers.expected_proxy_tag(host)
    for outbound in data.get("outbounds", []):
        if outbound.get("tag") == tag:
            pytest.fail(f"Unexpected outbound {tag} still present")


@pytest.mark.host
@pytest.mark.linux
def test_client_remove_endpoint_and_list(openwrt_host, xp2p_openwrt_ipk):
    runner = _prepare_host(openwrt_host, xp2p_openwrt_ipk)
    try:
        runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.66.0.10",
            "--user",
            "delta@example.com",
            "--password",
            "delta-pass",
            check=True,
        )
        runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.66.0.11",
            "--user",
            "echo@example.com",
            "--password",
            "echo-pass",
            check=True,
        )

        list_result = runner(
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
        runner(
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
        redirect_list = runner(
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

        runner(
            "client",
            "remove",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "10.66.0.10",
            "--quiet",
            check=True,
        )

        outbounds = helpers.read_json(openwrt_host, CLIENT_OUTBOUNDS)
        helpers.assert_outbound(
            outbounds,
            "10.66.0.11",
            "echo-pass",
            "echo@example.com",
            "10.66.0.11",
        )
        _assert_no_endpoint("10.66.0.10", outbounds)

        routing = helpers.read_json(openwrt_host, CLIENT_ROUTING)
        helpers.assert_routing_rule(routing, "10.66.0.11")

        state = helpers.read_json(openwrt_host, CLIENT_STATE_FILE)
        hosts = {entry.get("hostname") for entry in state.get("endpoints", [])}
        assert hosts == {"10.66.0.11"}

        redirect_list_after = runner(
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

        list_after = runner(
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

        runner(
            "client",
            "remove",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--all",
            "--quiet",
            check=True,
        )

        final_list = runner(
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
        helpers.cleanup_client_install(openwrt_host, runner)
        helpers.remove_path(openwrt_host, helpers.HEARTBEAT_STATE_FILE)


@pytest.mark.host
@pytest.mark.linux
def test_client_install_requires_force_for_duplicate_endpoint(openwrt_host, xp2p_openwrt_ipk):
    runner = _prepare_host(openwrt_host, xp2p_openwrt_ipk)
    try:
        runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.0.20",
            "--user",
            "state@example.com",
            "--password",
            "state-pass",
            check=True,
        )

        result = runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.0.20",
            "--user",
            "state2@example.com",
            "--password",
            "state-pass-2",
            check=False,
        )
        assert result.rc != 0, "Expected duplicate endpoint install to fail without --force"
        combined = f"{result.stdout}\n{result.stderr}".lower()
        assert "endpoint 10.55.0.20" in combined and "already exists" in combined

        runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.0.20",
            "--user",
            "state2@example.com",
            "--password",
            "state-pass-2",
            "--force",
            check=True,
        )
    finally:
        helpers.cleanup_client_install(openwrt_host, runner)
        helpers.remove_path(openwrt_host, helpers.HEARTBEAT_STATE_FILE)


@pytest.mark.host
@pytest.mark.linux
def test_client_install_recovers_without_state_marker(openwrt_host, xp2p_openwrt_ipk):
    runner = _prepare_host(openwrt_host, xp2p_openwrt_ipk)
    try:
        runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.0.30",
            "--user",
            "nostate@example.com",
            "--password",
            "nostate-pass",
            "--force",
            check=True,
        )

        for state_file in helpers.CLIENT_STATE_FILES:
            helpers.remove_path(openwrt_host, state_file)
            assert not helpers.path_exists(openwrt_host, state_file)

        runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.0.31",
            "--user",
            "nostate2@example.com",
            "--password",
            "nostate-pass-2",
            check=True,
        )

        assert any(helpers.path_exists(openwrt_host, path) for path in helpers.CLIENT_STATE_FILES), (
            "Expected client install-state markers to be recreated"
        )
    finally:
        helpers.cleanup_client_install(openwrt_host, runner)
        helpers.remove_path(openwrt_host, helpers.HEARTBEAT_STATE_FILE)
