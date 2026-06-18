from __future__ import annotations

import pytest

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env
from tests.host.tunnel import common as tunnel_common

CLIENT_STATE_FILE = helpers.CLIENT_CONFIG_FILE

pytestmark = [pytest.mark.host, pytest.mark.linux]

CLIENT_APPEND_OUTBOUNDS = helpers.CLIENT_CONFIG_DIR / "outbounds.append.json"
CLIENT_APPEND_INBOUNDS = helpers.CLIENT_CONFIG_DIR / "inbounds.append.json"
CLIENT_ROUTING_AFTER_MANAGED = helpers.CLIENT_CONFIG_DIR / "routing.rules.after-xp2p-managed.json"


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
    openwrt_env.install_ipk_on_host(openwrt_host, xp2p_openwrt_ipk)
    runner = _runner(openwrt_host)
    helpers.cleanup_client_install(openwrt_host, runner)
    helpers.remove_path(openwrt_host, helpers.HEARTBEAT_STATE_FILE)
    return runner


def _assert_endpoint(
    state: dict,
    host: str,
    password: str,
    user: str,
    server_name: str,
    *,
    allow_insecure: bool | None = None,
    pinned_peer_sha256: str | None = None,
    verify_peer_name: str | None = None,
) -> None:
    endpoints = state.get("endpoints", []) or []
    for endpoint in endpoints:
        if not isinstance(endpoint, dict):
            continue
        if (endpoint.get("hostname") or endpoint.get("address")) != host:
            continue
        assert endpoint.get("password") == password
        assert endpoint.get("user") == user
        assert endpoint.get("server_name") == server_name
        if allow_insecure is not None:
            assert bool(endpoint.get("allow_insecure")) is bool(allow_insecure)
        if pinned_peer_sha256 is not None:
            assert (endpoint.get("pinned_peer_cert_sha256") or "") == pinned_peer_sha256
        if verify_peer_name is not None:
            assert (endpoint.get("verify_peer_cert_by_name") or "") == verify_peer_name
        return
    raise AssertionError(f"Endpoint for host {host} not found in client state: {endpoints}")


def _assert_no_endpoint(host: str, state: dict) -> None:
    endpoints = state.get("endpoints", []) or []
    for endpoint in endpoints:
        if not isinstance(endpoint, dict):
            continue
        if (endpoint.get("hostname") or endpoint.get("address")) == host:
            pytest.fail(f"Unexpected endpoint for host {host} still present")


def _ensure_client_live_config(openwrt_host, runner) -> None:
    runner("client", "service", "start", check=True)
    helpers.wait_for_apply_request_clear(openwrt_host, timeout_seconds=60.0)
    helpers.wait_for_live_config(openwrt_host, "client", timeout_seconds=60.0)


@pytest.mark.host
@pytest.mark.linux
def test_client_install_default_creates_tun_inbound(openwrt_host, xp2p_openwrt_ipk):
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
            "10.55.0.12",
            "--user",
            "tun-default@example.com",
            "--password",
            "tun-default-pass",
            check=True,
        )

        state = helpers.read_preferred_client_config(openwrt_host)
        assert state.get("tun_enabled") is True
        assert state.get("tun_name") == "xp2pc"
        tun = (((state.get("xray") or {}).get("inbounds") or {}).get("tun")) or {}
        assert tun.get("protocol") == "tun"
    finally:
        helpers.cleanup_client_install(openwrt_host, runner)
        helpers.remove_path(openwrt_host, helpers.HEARTBEAT_STATE_FILE)


@pytest.mark.host
@pytest.mark.linux
def test_client_install_respects_tun_disabled(openwrt_host, xp2p_openwrt_ipk):
    runner = _prepare_host(openwrt_host, xp2p_openwrt_ipk)
    try:
        result = openwrt_env.run_xp2p_with_env(
            openwrt_host,
            {"XP2P_CLIENT_TUN_ENABLED": "false"},
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.0.13",
            "--user",
            "tun-disabled@example.com",
            "--password",
            "tun-disabled-pass",
            "--force",
        )
        if result.rc != 0:
            pytest.fail(
                "xp2p command failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )

        state = helpers.read_preferred_client_config(openwrt_host)
        assert state.get("tun_enabled") is False
    finally:
        helpers.cleanup_client_install(openwrt_host, runner)
        helpers.remove_path(openwrt_host, helpers.HEARTBEAT_STATE_FILE)


@pytest.mark.host
@pytest.mark.linux
def test_client_install_mode_proxy_sets_tun_disabled(openwrt_host, xp2p_openwrt_ipk):
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
            "10.55.0.14",
            "--user",
            "mode-proxy@example.com",
            "--password",
            "mode-proxy-pass",
            "--mode",
            "proxy",
            "--force",
            check=True,
        )

        state = helpers.read_preferred_client_config(openwrt_host)
        assert state.get("tun_enabled") is False
    finally:
        helpers.cleanup_client_install(openwrt_host, runner)
        helpers.remove_path(openwrt_host, helpers.HEARTBEAT_STATE_FILE)


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

        state = helpers.read_preferred_client_config(openwrt_host)
        _assert_endpoint(state, "10.55.0.10", "test_password123", "alpha@example.com", "10.55.0.10")

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

        state = helpers.read_preferred_client_config(openwrt_host)
        _assert_endpoint(
            state,
            "10.55.0.10",
            "test_password123",
            "alpha@example.com",
            "10.55.0.10",
            allow_insecure=False,
        )
        _assert_endpoint(
            state,
            "10.55.0.11",
            "override_password456",
            "beta@example.com",
            "vpn.example.local",
            allow_insecure=False,
        )
        _ensure_client_live_config(openwrt_host, runner)
        routing = helpers.read_live_json(openwrt_host, helpers.CLIENT_CONFIG_DIR / "routing.json")
        helpers.assert_routing_rule(routing, "10.55.0.10")
        helpers.assert_routing_rule(routing, "10.55.0.11")

        state = helpers.read_preferred_client_config(openwrt_host)
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

        state = helpers.read_preferred_client_config(openwrt_host)
        _assert_endpoint(
            state,
            "10.55.0.10",
            "forcepass",
            "gamma@example.com",
            "override.linux",
            allow_insecure=False,
        )
        _assert_endpoint(
            state,
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
        link_host = "10.55.0.99"
        link = (
            f"trojan://linkpass@{link_host}:58443?"
            "pinnedPeerCertSha256=deadbeef&security=tls&sni=link.example.test&"
            "verifyPeerCertByName=link.example.test#link@example.com"
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
        state = helpers.read_preferred_client_config(openwrt_host)
        _assert_endpoint(
            state,
            link_host,
            "linkpass",
            "link@example.com",
            "link.example.test",
            pinned_peer_sha256="deadbeef",
            verify_peer_name="link.example.test",
        )
    finally:
        helpers.cleanup_client_install(openwrt_host, runner)
        helpers.remove_path(openwrt_host, helpers.HEARTBEAT_STATE_FILE)


@pytest.mark.host
@pytest.mark.linux
def test_client_install_from_link_without_allow_insecure(openwrt_host, xp2p_openwrt_ipk):
    runner = _prepare_host(openwrt_host, xp2p_openwrt_ipk)
    try:
        link_host = "10.55.0.98"
        link = (
            f"trojan://linkpass@{link_host}:58443?"
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
        state = helpers.read_preferred_client_config(openwrt_host)
        _assert_endpoint(
            state,
            link_host,
            "linkpass",
            "link@example.com",
            "link.example.test",
            allow_insecure=False,
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
        link_host = "10.55.0.97"
        link = (
            f"trojan://statepass@{link_host}:58443?"
            "pinnedPeerCertSha256=deadbeef&security=tls&sni=link.example.test&"
            "verifyPeerCertByName=link.example.test#state-two@example.com"
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
            "--pending",
            check=True,
        )
        rows = tunnel_common.parse_state_rows(result.stdout or "")
        expected_tags = {
            helpers.expected_proxy_tag("10.55.0.40"),
            helpers.expected_proxy_tag(link_host),
        }
        expected_hosts = {"10.55.0.40", link_host}
        assert len(rows) == 2
        assert {row["TAG"] for row in rows} == expected_tags
        assert {row["HOST"] for row in rows} == expected_hosts
    finally:
        helpers.cleanup_client_install(openwrt_host, runner)
        helpers.remove_path(openwrt_host, helpers.HEARTBEAT_STATE_FILE)


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
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            check=True,
        ).stdout or ""
        assert redirect_cidr in redirect_list

        runner("client", "service", "stop")
        openwrt_host.run("/etc/init.d/xp2p-client disable >/dev/null 2>&1 || true")
        openwrt_host.run("/etc/init.d/xp2p-client stop >/dev/null 2>&1 || true")
        openwrt_env.run_guest_script(openwrt_host, "scripts/linux/kill_xp2p_processes.sh")
        helpers.wait_for_service_state(openwrt_host, "client", expected_active=False, timeout_seconds=30.0)
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

        state = helpers.read_preferred_client_config(openwrt_host)
        _assert_endpoint(state, "10.66.0.11", "echo-pass", "echo@example.com", "10.66.0.11")
        _assert_no_endpoint("10.66.0.10", state)

        state = helpers.read_preferred_client_config(openwrt_host)
        hosts = {entry.get("hostname") for entry in state.get("endpoints", [])}
        assert hosts == {"10.66.0.11"}

        redirect_list_after = runner(
            "client",
            "redirect",
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

        assert helpers.path_exists(openwrt_host, helpers.CLIENT_CONFIG_FILE), (
            "Expected client config to be recreated"
        )
        assert helpers.path_exists(openwrt_host, helpers.CLIENT_CONFIG_DIR), "Expected client config directory to exist"
        assert helpers.path_exists(openwrt_host, CLIENT_APPEND_OUTBOUNDS), "Expected outbounds append file to exist"
        assert helpers.path_exists(openwrt_host, CLIENT_APPEND_INBOUNDS), "Expected inbounds append file to exist"
        assert helpers.path_exists(openwrt_host, CLIENT_ROUTING_AFTER_MANAGED), (
            "Expected managed routing rules file to exist"
        )
    finally:
        helpers.cleanup_client_install(openwrt_host, runner)
        helpers.remove_path(openwrt_host, helpers.HEARTBEAT_STATE_FILE)
