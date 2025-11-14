from __future__ import annotations

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import env as linux_env

STATE_FILES = {
    "client": helpers.INSTALL_ROOT / "install-state-client.json",
    "server": helpers.INSTALL_ROOT / "install-state-server.json",
}
LEGACY_STATE = helpers.INSTALL_ROOT / "install-state.json"


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
            roles[role] = linux_env.read_json(host, path)
    if roles:
        return roles
    if linux_env.path_exists(host, LEGACY_STATE):
        data = linux_env.read_json(host, LEGACY_STATE)
        nested = data.get("roles")
        if nested:
            return nested
        if kind := data.get("kind"):
            roles[kind] = data
    return roles


@pytest.mark.host
@pytest.mark.linux
def test_client_and_server_share_install_dir(server_host):
    run = lambda *cmd, check=False: _xp2p_run(server_host, *cmd, check=check)
    try:
        helpers.cleanup_client_install(server_host, run)
        helpers.cleanup_server_install(server_host, run)

        run(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--server-address",
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
            "--ignore-missing",
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
        )
        run(
            "client",
            "remove",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--ignore-missing",
        )
