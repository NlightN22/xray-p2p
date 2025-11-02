import json
from pathlib import Path

import pytest

from tests.host import _env

CLIENT_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
CLIENT_CONFIG_DIR_NAME = "config-client"
CLIENT_CONFIG_DIR = CLIENT_INSTALL_DIR / CLIENT_CONFIG_DIR_NAME
CLIENT_CONFIG_OUTBOUNDS = CLIENT_CONFIG_DIR / "outbounds.json"
CLIENT_LOG_RELATIVE = r"logs\client.err"
CLIENT_LOG_FILE = CLIENT_INSTALL_DIR / CLIENT_LOG_RELATIVE


def _cleanup_client_install(runner) -> None:
    runner("client", "remove", "--ignore-missing")
    _env.prepare_program_files_install()


def _read_remote_json(client_host, path: Path) -> dict:
    quoted = _env.ps_quote(str(path))
    script = f"""
$ErrorActionPreference = 'Stop'
if (-not (Test-Path {quoted})) {{
    exit 3
}}
Get-Content -Raw {quoted}
"""
    result = _env.run_powershell(client_host, script)
    assert result.rc == 0, (
        f"Failed to read remote JSON {path}:\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
    )
    try:
        return json.loads(result.stdout)
    except json.JSONDecodeError as exc:
        pytest.fail(f"Failed to parse JSON from {path}: {exc}\nContent:\n{result.stdout}")


def _remote_path_exists(client_host, path: Path) -> bool:
    quoted = _env.ps_quote(str(path))
    script = f"if (Test-Path {quoted}) {{ exit 0 }} else {{ exit 3 }}"
    result = _env.run_powershell(client_host, script)
    return result.rc == 0


@pytest.mark.host
def test_client_install_and_force_overwrites(client_host, xp2p_client_runner):
    _cleanup_client_install(xp2p_client_runner)
    try:
        xp2p_client_runner(
            "client",
            "install",
            "--server-address",
            "10.0.10.10",
            "--password",
            "test_password123",
            "--force",
            check=True,
        )

        data = _read_remote_json(client_host, CLIENT_CONFIG_OUTBOUNDS)
        primary = data["outbounds"][0]["settings"]["servers"][0]
        assert primary["address"] == "10.0.10.10"
        assert primary["password"] == "test_password123"
        assert data["outbounds"][0]["streamSettings"]["tlsSettings"]["serverName"] == "10.0.10.10"

        xp2p_client_runner(
            "client",
            "install",
            "--server-address",
            "10.0.10.11",
            "--password",
            "override_password456",
            "--server-name",
            "vpn.example.local",
            "--force",
            check=True,
        )

        updated_data = _read_remote_json(client_host, CLIENT_CONFIG_OUTBOUNDS)
        primary_updated = updated_data["outbounds"][0]["settings"]["servers"][0]
        assert primary_updated["address"] == "10.0.10.11"
        assert primary_updated["password"] == "override_password456"
        assert (
            updated_data["outbounds"][0]["streamSettings"]["tlsSettings"]["serverName"]
            == "vpn.example.local"
        )
    finally:
        _cleanup_client_install(xp2p_client_runner)


@pytest.mark.host
def test_client_run_starts_xray_core(client_host, xp2p_client_runner, xp2p_client_run_factory):
    _cleanup_client_install(xp2p_client_runner)
    try:
        xp2p_client_runner(
            "client",
            "install",
            "--server-address",
            "10.0.10.10",
            "--password",
            "runtime_password789",
            "--force",
            check=True,
        )

        with xp2p_client_run_factory(
            str(CLIENT_INSTALL_DIR), CLIENT_CONFIG_DIR_NAME, CLIENT_LOG_RELATIVE
        ) as session:
            assert session["pid"] > 0

        assert _remote_path_exists(client_host, CLIENT_LOG_FILE), (
            f"Expected log file {CLIENT_LOG_FILE} to be created"
        )
        log_content = _env.run_powershell(
            client_host,
            f"$ErrorActionPreference='Stop'; Get-Content -Raw {_env.ps_quote(str(CLIENT_LOG_FILE))}",
        ).stdout
        assert log_content.strip(), "Expected xray-core to produce log output"
        assert "Failed to start" not in log_content
    finally:
        _cleanup_client_install(xp2p_client_runner)
