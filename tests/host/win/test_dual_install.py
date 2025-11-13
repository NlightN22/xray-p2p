from __future__ import annotations

import json
from pathlib import Path

import pytest

from tests.host.win import env as _env

INSTALL_DIR = Path(r"C:\Program Files\xp2p")
CLIENT_CONFIG_DIR = "config-client"
SERVER_CONFIG_DIR = "config-server"
XRAY_BINARY = INSTALL_DIR / "bin" / "xray.exe"
STATE_FILES = {
    "client": INSTALL_DIR / "install-state-client.json",
    "server": INSTALL_DIR / "install-state-server.json",
}
LEGACY_STATE_FILE = INSTALL_DIR / "install-state.json"


def _xp2p_run(host, *args: str, check: bool = False):
    result = _env.run_xp2p(host, args)
    stdout = result.stdout or ""
    if "__XP2P_MISSING__" in stdout:
        pytest.skip(
            f"xp2p.exe not found on {_env.DEFAULT_SERVER} at {_env.XP2P_EXE}. "
            "Provision the guest before running host tests."
        )
    if check and result.rc != 0:
        pytest.fail(
            "xp2p command failed on "
            f"{_env.DEFAULT_SERVER} (exit {result.rc}).\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return result


def _read_remote_json(host, path: Path) -> dict:
    quoted = _env.ps_quote(str(path))
    script = f"""
$ErrorActionPreference = 'Stop'
if (-not (Test-Path {quoted})) {{
    exit 3
}}
Get-Content -Raw {quoted}
"""
    result = _env.run_powershell(host, script)
    assert result.rc == 0, (
        f"Failed to read remote JSON {path}:\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
    )
    return json.loads(result.stdout)


def _read_remote_json_optional(host, path: Path) -> tuple[dict | None, bool]:
    quoted = _env.ps_quote(str(path))
    script = f"""
$ErrorActionPreference = 'Stop'
if (-not (Test-Path {quoted})) {{
    exit 3
}}
Get-Content -Raw {quoted}
"""
    result = _env.run_powershell(host, script)
    if result.rc == 3:
        return None, False
    if result.rc != 0:
        return None, False
    return json.loads(result.stdout), True


def _read_roles(host) -> dict:
    roles: dict[str, dict] = {}
    for role, path in STATE_FILES.items():
        data, ok = _read_remote_json_optional(host, path)
        if ok and data is not None:
            roles[role] = data
    if roles:
        return roles
    legacy, ok = _read_remote_json_optional(host, LEGACY_STATE_FILE)
    if not ok or legacy is None:
        return roles
    if nested := legacy.get("roles"):
        return nested
    if kind := legacy.get("kind"):
        roles[kind] = legacy
    return roles


def _remote_sha256(host, path: Path) -> str:
    quoted = _env.ps_quote(str(path))
    script = f"""
$ErrorActionPreference = 'Stop'
if (-not (Test-Path {quoted})) {{
    exit 3
}}
(Get-FileHash -Algorithm SHA256 {quoted}).Hash
"""
    result = _env.run_powershell(host, script)
    assert result.rc == 0, (
        f"Failed to hash remote file {path}:\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
    )
    return (result.stdout or "").strip().lower()


@pytest.mark.host
@pytest.mark.win
def test_client_and_server_share_install_dir(server_host, xp2p_msi_path):
    _env.install_xp2p_from_msi(server_host, xp2p_msi_path)

    def run(*cmd: str, check: bool = False):
        return _xp2p_run(server_host, *cmd, check=check)

    try:
        run(
            "client",
            "remove",
            "--path",
            str(INSTALL_DIR),
            "--config-dir",
            CLIENT_CONFIG_DIR,
            "--ignore-missing",
            check=True,
        )
        run(
            "server",
            "remove",
            "--path",
            str(INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR,
            "--ignore-missing",
            check=True,
        )

        run(
            "client",
            "install",
            "--path",
            str(INSTALL_DIR),
            "--config-dir",
            CLIENT_CONFIG_DIR,
            "--server-address",
            "10.0.10.210",
            "--user",
            "dual@example.com",
            "--password",
            "dual-pass",
            "--force",
            check=True,
        )
        client_hash = _remote_sha256(server_host, XRAY_BINARY)

        run(
            "server",
            "install",
            "--path",
            str(INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR,
            "--port",
            "62444",
            "--host",
            "dual.xp2p.test",
            check=True,
        )
        server_hash = _remote_sha256(server_host, XRAY_BINARY)
        assert client_hash == server_hash, "Expected shared xray.exe to be reused without modification"

        roles = _read_roles(server_host)
        assert "client" in roles and "server" in roles, f"Unexpected roles state: {roles}"

        run(
            "client",
            "remove",
            "--path",
            str(INSTALL_DIR),
            "--config-dir",
            CLIENT_CONFIG_DIR,
            "--ignore-missing",
            check=True,
        )
        roles_after = _read_roles(server_host)
        assert "server" in roles_after and "client" not in roles_after, (
            f"Client removal should not delete server role: {roles_after}"
        )
    finally:
        run(
            "server",
            "remove",
            "--path",
            str(INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR,
            "--ignore-missing",
        )
        run(
            "client",
            "remove",
            "--path",
            str(INSTALL_DIR),
            "--config-dir",
            CLIENT_CONFIG_DIR,
            "--ignore-missing",
        )
