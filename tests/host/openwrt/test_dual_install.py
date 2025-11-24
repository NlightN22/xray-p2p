from __future__ import annotations

import pytest

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env

STATE_FILES = {
    "client": helpers.INSTALL_ROOT / "install-state-client.json",
    "server": helpers.INSTALL_ROOT / "install-state-server.json",
}
LEGACY_STATE = helpers.INSTALL_ROOT / "install-state.json"


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
            roles[role] = helpers.read_json(host, path)
    if roles:
        return roles
    if helpers.path_exists(host, LEGACY_STATE):
        data = helpers.read_json(host, LEGACY_STATE)
        nested = data.get("roles")
        if nested:
            return nested
        if kind := data.get("kind"):
            roles[kind] = data
    return roles


@pytest.mark.host
@pytest.mark.linux
def test_openwrt_client_and_server_share_install_dir(openwrt_host, xp2p_openwrt_ipk):
    openwrt_env.sync_build_output(openwrt_env.DEFAULT_OPENWRT_MACHINE)
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
def test_openwrt_client_and_server_install_support_extended_arguments(openwrt_host, xp2p_openwrt_ipk):
    openwrt_env.sync_build_output(openwrt_env.DEFAULT_OPENWRT_MACHINE)
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
            "--allow-insecure",
            "--force",
            check=True,
        )

        client_config_path = helpers.INSTALL_ROOT / custom_client_config
        outbounds = helpers.read_json(openwrt_host, client_config_path / "outbounds.json")
        helpers.assert_outbound(
            outbounds,
            client_host,
            client_password,
            client_user,
            client_sni,
            allow_insecure=True,
        )
        routing = helpers.read_json(openwrt_host, client_config_path / "routing.json")
        helpers.assert_routing_rule(routing, client_host)
        state = helpers.read_first_existing_json(openwrt_host, helpers.CLIENT_STATE_FILES)
        recorded_hosts = {entry.get("hostname") for entry in state.get("endpoints", [])}
        assert recorded_hosts == {client_host}

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

        server_roles = _read_roles(openwrt_host)
        assert {"client", "server"} <= set(server_roles.keys()), f"Unexpected roles: {server_roles}"
        custom_server_path = helpers.INSTALL_ROOT / custom_server_config
        assert helpers.path_exists(
            openwrt_host, custom_server_path / "inbounds.json"
        ), "Server config directory missing expected inbounds.json"
    finally:
        helpers.cleanup_server_install(openwrt_host, run, config_dir=custom_server_config)
        helpers.cleanup_client_install(openwrt_host, run, config_dir=custom_client_config)
