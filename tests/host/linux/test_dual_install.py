from __future__ import annotations

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import env as linux_env

STATE_FILES = {
    "client": helpers.CLIENT_CONFIG_FILE,
    "server": helpers.SERVER_CONFIG_FILE,
}


def _xp2p_run(host, *args: str, check: bool = False):
    result = linux_env.run_xp2p(host, *args)
    if check and result.rc != 0:
        pytest.fail(
            "xp2p command failed "
            f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return result


def _read_roles(host) -> dict:
    roles: dict[str, dict] = {}
    for role, path in STATE_FILES.items():
        if linux_env.path_exists(host, path):
            roles[role] = helpers.read_toml(host, path).get(role) or {}
    return roles


@pytest.mark.host
@pytest.mark.linux
def test_client_and_server_share_install_dir(server_host):
    run = lambda *cmd, check=False: _xp2p_run(server_host, *cmd, check=check)
    try:

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
        client_hash = helpers.file_sha256(server_host, helpers.XRAY_BINARY)

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
        server_hash = helpers.file_sha256(server_host, helpers.XRAY_BINARY)
        assert client_hash == server_hash

        roles = _read_roles(server_host)
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
        roles_after = _read_roles(server_host)
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
def test_client_and_server_install_support_extended_arguments(server_host):
    run = lambda *cmd, check=False: _xp2p_run(server_host, *cmd, check=check)
    custom_client_config = "config-client-max"
    custom_server_config = "config-server-max"
    client_host = "10.66.1.56"
    client_port = "62089"
    client_user = "max_linux@example.com"
    client_password = "max-linux-pass"
    client_sni = "edge.linux.example"
    server_host_value = "linux-max.example"
    server_port_value = "62577"
    try:

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

        client_config_path = helpers.INSTALL_ROOT / custom_client_config
        outbounds = helpers.read_json(server_host, client_config_path / "outbounds.json")
        helpers.assert_outbound(
            outbounds,
            client_host,
            client_password,
            client_user,
            client_sni,
        )
        routing = helpers.read_json(server_host, client_config_path / "routing.json")
        helpers.assert_routing_rule(routing, client_host)
        client_state = helpers.read_client_config(server_host)
        recorded_hosts = {entry.get("hostname") for entry in client_state.get("endpoints", [])}
        assert recorded_hosts == {client_host}

        run(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            custom_server_config,
            "--port",
            server_port_value,
            "--host",
            server_host_value,
            "--force",
            check=True,
        )

        roles = _read_roles(server_host)
        assert {"client", "server"} <= set(roles.keys())
    finally:
        pass
