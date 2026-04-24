from __future__ import annotations

import pytest

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env

STATE_FILES = {
    "client": helpers.CLIENT_CONFIG_FILE,
    "server": helpers.SERVER_CONFIG_FILE,
}


def _xp2p_run(host, *args: str, check: bool = False):
    result = openwrt_env.run_xp2p(host, *args)
    if check and result.rc != 0:
        pytest.fail(
            "xp2p command failed "
            f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return result


def _read_roles(host) -> dict:
    roles: dict[str, dict] = {}
    for role, path in STATE_FILES.items():
        if helpers.path_exists(host, path):
            roles[role] = helpers.read_toml(host, path).get(role) or {}
    return roles


def _update_hosts_entry(host, action: str, domain: str, ip: str | None = None) -> None:
    args = ["scripts/linux/update_hosts_entry_openwrt.sh", action]
    if action == "add":
        if not ip:
            raise AssertionError("IP is required for add action")
        args.extend([ip, domain])
    else:
        args.append(domain)
    result = openwrt_env.run_guest_script(host, *args)
    if result.rc != 0:
        pytest.fail(
            "guest script failed "
            f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def _extract_link(output: str) -> str:
    for raw in (output or "").splitlines():
        stripped = raw.strip()
        if stripped.startswith("trojan://"):
            return stripped
    pytest.fail(f"xp2p server user add did not emit trojan link.\nSTDOUT:\n{output}")


def _ensure_pinned_peer(link: str) -> str:
    if "pinnedPeerCertSha256=" not in link:
        pytest.fail(f"Expected pinnedPeerCertSha256 in link: {link}")
    return link


@pytest.mark.host
@pytest.mark.linux
def test_openwrt_client_and_server_share_install_dir(openwrt_host, xp2p_openwrt_ipk):
    openwrt_env.install_ipk_on_host(openwrt_host, xp2p_openwrt_ipk, force=True)
    run = lambda *cmd, check=False: _xp2p_run(openwrt_host, *cmd, check=check)
    try:
        helpers.cleanup_client_install(openwrt_host, run)
        helpers.cleanup_server_install(openwrt_host, run)

        run(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.66.0.10",
            "--user",
            "dual@example.com",
            "--password",
            "dual-pass",
            "--force",
            check=True,
        )
        client_hash = helpers.file_sha256(openwrt_host, helpers.XRAY_BINARY)

        run(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--port",
            "62555",
            "--host",
            "dual.xp2p.test",
            check=True,
        )
        server_hash = helpers.file_sha256(openwrt_host, helpers.XRAY_BINARY)
        assert client_hash == server_hash

        roles = _read_roles(openwrt_host)
        assert "client" in roles and "server" in roles, f"Unexpected role state: {roles}"

        run(
            "client",
            "remove",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--all",
            "--ignore-missing",
            "--quiet",
            check=True,
        )
        roles_after = _read_roles(openwrt_host)
        assert "server" in roles_after and "client" not in roles_after
    finally:
        run(
            "server",
            "remove",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--ignore-missing",
            "--quiet",
        )
        run(
            "client",
            "remove",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--all",
            "--ignore-missing",
            "--quiet",
        )


@pytest.mark.host
@pytest.mark.linux
def test_openwrt_dual_install_uses_distinct_tun_interfaces(openwrt_host, xp2p_openwrt_ipk):
    openwrt_env.install_ipk_on_host(openwrt_host, xp2p_openwrt_ipk, force=True)
    run = lambda *cmd, check=False: _xp2p_run(openwrt_host, *cmd, check=check)
    try:
        helpers.cleanup_client_install(openwrt_host, run)
        helpers.cleanup_server_install(openwrt_host, run)

        run(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.66.0.20",
            "--user",
            "dual-tun@example.com",
            "--password",
            "dual-tun-pass",
            "--force",
            check=True,
        )
        result = openwrt_env.run_xp2p_with_env(
            openwrt_host,
            {"XP2P_SERVER_TUN_ENABLED": "true"},
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--port",
            "62557",
            "--host",
            "dual-tun.openwrt.test",
            "--force",
        )
        if result.rc != 0:
            pytest.fail(
                "xp2p command failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )

        client_state = helpers.read_preferred_client_config(openwrt_host)
        server_state = helpers.read_preferred_server_config(openwrt_host)
        assert client_state.get("tun_enabled") is True
        assert client_state.get("tun_name") == "xp2pc"
        assert server_state.get("tun_enabled") is True
        assert server_state.get("tun_name") == "xp2ps"
    finally:
        helpers.cleanup_server_install(openwrt_host, run)
        helpers.cleanup_client_install(openwrt_host, run)


@pytest.mark.host
@pytest.mark.linux
def test_openwrt_client_and_server_install_support_extended_arguments(openwrt_host, xp2p_openwrt_ipk):
    openwrt_env.install_ipk_on_host(openwrt_host, xp2p_openwrt_ipk, force=True)
    run = lambda *cmd, check=False: _xp2p_run(openwrt_host, *cmd, check=check)
    custom_client_config = "config-client-max"
    custom_server_config = "config-server-max"
    client_host = "10.66.1.55"
    client_port = "62088"
    client_user = "max@example.com"
    client_password = "max-pass-123"
    client_sni = "edge.openwrt.test"
    server_host = "max-server.openwrt.test"
    server_port = "62566"
    try:
        helpers.cleanup_client_install(openwrt_host, run, config_dir=custom_client_config)
        helpers.cleanup_server_install(openwrt_host, run, config_dir=custom_server_config)

        run(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            custom_client_config,
            "--host",
            client_host,
            "--port",
            client_port,
            "--user",
            client_user,
            "--password",
            client_password,
            "--sni",
            client_sni,
            "--force",
            check=True,
        )

        state = helpers.read_preferred_client_config(openwrt_host)
        recorded_hosts = {entry.get("hostname") for entry in state.get("endpoints", [])}
        assert recorded_hosts == {client_host}
        helpers.ensure_service_running(openwrt_host, "client")
        helpers.wait_for_apply_request_clear(openwrt_host, timeout_seconds=60.0)
        helpers.wait_for_live_config(openwrt_host, "client")
        routing = helpers.read_live_json(openwrt_host, helpers.CLIENT_CONFIG_DIR / "routing.json")
        helpers.assert_routing_rule(routing, client_host)

        run(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            custom_server_config,
            "--port",
            server_port,
            "--host",
            server_host,
            "--force",
            check=True,
        )

        helpers.ensure_service_running(openwrt_host, "server")
        helpers.wait_for_apply_request_clear(openwrt_host, timeout_seconds=60.0)
        helpers.wait_for_live_config(openwrt_host, "server")
        server_roles = _read_roles(openwrt_host)
        assert {"client", "server"} <= set(server_roles.keys()), f"Unexpected roles: {server_roles}"
        assert helpers.path_exists(
            openwrt_host, helpers.SERVER_LIVE_DIR / "runtime.json"
        ), "Server live runtime.json missing after install"
        assert helpers.path_exists(
            openwrt_host, helpers.SERVER_LIVE_DIR / "xray.json"
        ), "Server live xray.json missing after install"
    finally:
        helpers.cleanup_server_install(openwrt_host, run, config_dir=custom_server_config)
        helpers.cleanup_client_install(openwrt_host, run, config_dir=custom_client_config)


@pytest.mark.host
@pytest.mark.linux
def test_openwrt_client_and_server_states_are_isolated(openwrt_host, xp2p_openwrt_ipk):
    openwrt_env.install_ipk_on_host(openwrt_host, xp2p_openwrt_ipk, force=True)
    run = lambda *cmd, check=False: _xp2p_run(openwrt_host, *cmd, check=check)
    server_domain_a = "srv-a.local"
    server_domain_b = "srv-b.local"
    server_port = "62070"
    user_b = "state-b@example.com"
    pass_b = "state-pass-b"
    try:
        helpers.cleanup_client_install(openwrt_host, run)
        helpers.cleanup_server_install(openwrt_host, run)
        primary_ip = helpers.detect_primary_ipv4(openwrt_host)
        _update_hosts_entry(openwrt_host, "add", server_domain_a, primary_ip)
        _update_hosts_entry(openwrt_host, "add", server_domain_b, primary_ip)

        server_install = run(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--port",
            server_port,
            "--host",
            server_domain_a,
            "--force",
            check=True,
        )
        default_cred = helpers.extract_trojan_credential(server_install.stdout or "")
        link_a = _ensure_pinned_peer(default_cred["link"])

        user_add = run(
            "server",
            "user",
            "add",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--id",
            user_b,
            "--password",
            pass_b,
            "--host",
            server_domain_b,
            check=True,
        )
        link_b = _ensure_pinned_peer(_extract_link(user_add.stdout or ""))

        client_install_a = run(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--link",
            link_a,
            "--force",
            check=True,
        )
        client_install_b = run(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--link",
            link_b,
            check=True,
        )
        expected_tags = {
            helpers.expected_proxy_tag(server_domain_a),
            helpers.expected_proxy_tag(server_domain_b),
        }
        expected_users = {default_cred["user"], user_b}

        client_state = helpers.read_preferred_client_config(openwrt_host)
        recorded_hosts = {entry.get("hostname") for entry in client_state.get("endpoints", [])}
        recorded_tags = {entry.get("tag") for entry in client_state.get("endpoints", [])}
        recorded_users = {entry.get("user") for entry in client_state.get("endpoints", [])}
        assert recorded_hosts == {server_domain_a, server_domain_b}
        assert recorded_tags == expected_tags
        assert recorded_users == expected_users

        server_state = helpers.read_preferred_server_config(openwrt_host)
        reverse_channels = server_state.get("reverse_channels", {})
        if not isinstance(reverse_channels, dict):
            pytest.fail("Server config missing reverse_channels map")
        recorded_server_users = {
            entry.get("user_id")
            for entry in reverse_channels.values()
            if isinstance(entry, dict)
        }
        assert recorded_server_users == expected_users

        added_links = {link_a, link_b}
        installed_links = {
            default_cred["link"],
            _extract_link(user_add.stdout or ""),
        }
        assert installed_links == added_links, (
            "Client install links should match server user links.\n"
            f"client: {client_install_a.stdout}\n{client_install_b.stdout}\n"
            f"server: {server_install.stdout}\n{user_add.stdout}"
        )
    finally:
        _update_hosts_entry(openwrt_host, "remove", server_domain_a)
        _update_hosts_entry(openwrt_host, "remove", server_domain_b)
        helpers.cleanup_server_install(openwrt_host, run)
        helpers.cleanup_client_install(openwrt_host, run)
