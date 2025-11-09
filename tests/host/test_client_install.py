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
CLIENT_STATE_FILE = CLIENT_INSTALL_DIR / "install-state.json"


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


def _remove_remote_path(client_host, path: Path) -> None:
    quoted = _env.ps_quote(str(path))
    script = f"""
$ErrorActionPreference = 'Stop'
if (Test-Path {quoted}) {{
    Remove-Item {quoted} -Force -Recurse -ErrorAction SilentlyContinue
}}
"""
    _env.run_powershell(client_host, script)


def _assert_outbounds_server(data: dict, address: str, password: str, email: str, server_name: str) -> None:
    primary = data["outbounds"][0]["settings"]["servers"][0]
    assert primary["address"] == address
    assert primary["password"] == password
    assert primary["email"] == email
    tls_settings = data["outbounds"][0]["streamSettings"]["tlsSettings"]
    assert tls_settings["serverName"] == server_name


@pytest.mark.host
def test_client_install_and_force_overwrites(client_host, xp2p_client_runner):
    _cleanup_client_install(xp2p_client_runner)
    try:
        xp2p_client_runner(
            "client",
            "install",
            "--server-address",
            "10.0.10.10",
            "--user",
            "alpha@example.com",
            "--password",
            "test_password123",
            "--force",
            check=True,
        )

        data = _read_remote_json(client_host, CLIENT_CONFIG_OUTBOUNDS)
        _assert_outbounds_server(data, "10.0.10.10", "test_password123", "alpha@example.com", "10.0.10.10")

        xp2p_client_runner(
            "client",
            "install",
            "--server-address",
            "10.0.10.11",
            "--user",
            "beta@example.com",
            "--password",
            "override_password456",
            "--server-name",
            "vpn.example.local",
            "--force",
            check=True,
        )

        updated_data = _read_remote_json(client_host, CLIENT_CONFIG_OUTBOUNDS)
        _assert_outbounds_server(
            updated_data, "10.0.10.11", "override_password456", "beta@example.com", "vpn.example.local"
        )
    finally:
        _cleanup_client_install(xp2p_client_runner)


@pytest.mark.host
def test_client_install_from_link(client_host, xp2p_client_runner):
    _cleanup_client_install(xp2p_client_runner)
    try:
        link = (
            "trojan://linkpass@link.example.test:62022?"
            "allowInsecure=1&security=tls&sni=link.example.test#link@example.com"
        )
        xp2p_client_runner(
            "client",
            "install",
            "--link",
            link,
            "--force",
            check=True,
        )

        data = _read_remote_json(client_host, CLIENT_CONFIG_OUTBOUNDS)
        _assert_outbounds_server(
            data, "link.example.test", "linkpass", "link@example.com", "link.example.test"
        )
        tls_settings = data["outbounds"][0]["streamSettings"]["tlsSettings"]
        assert tls_settings["allowInsecure"] is True
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
            "--user",
            "gamma@example.com",
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


@pytest.mark.host
def test_client_install_requires_force_when_state_exists(client_host, xp2p_client_runner):
    _cleanup_client_install(xp2p_client_runner)
    try:
        xp2p_client_runner(
            "client",
            "install",
            "--server-address",
            "10.0.10.50",
            "--user",
            "state@example.com",
            "--password",
            "state-pass",
            "--force",
            check=True,
        )

        result = xp2p_client_runner(
            "client",
            "install",
            "--server-address",
            "10.0.10.51",
            "--user",
            "state2@example.com",
            "--password",
            "state-pass-2",
            check=False,
        )
        assert result.rc != 0, "Expected install to fail while state file exists"
        combined = f"{result.stdout}\n{result.stderr}".strip().lower()
        assert "client already installed" in combined, f"Unexpected error output:\n{result.stdout}\n{result.stderr}"
    finally:
        _cleanup_client_install(xp2p_client_runner)


@pytest.mark.host
def test_client_install_succeeds_without_state_marker(client_host, xp2p_client_runner):
    _cleanup_client_install(xp2p_client_runner)
    try:
        xp2p_client_runner(
            "client",
            "install",
            "--server-address",
            "10.0.10.60",
            "--user",
            "nostate@example.com",
            "--password",
            "nostate-pass",
            "--force",
            check=True,
        )

        _remove_remote_path(client_host, CLIENT_STATE_FILE)
        assert not _remote_path_exists(
            client_host, CLIENT_STATE_FILE
        ), "Expected install-state.json to be removed before re-install"

        xp2p_client_runner(
            "client",
            "install",
            "--server-address",
            "10.0.10.61",
            "--user",
            "nostate2@example.com",
            "--password",
            "nostate-pass-2",
            check=True,
        )

        assert _remote_path_exists(client_host, CLIENT_STATE_FILE), "Expected install-state.json to be recreated"
    finally:
        _cleanup_client_install(xp2p_client_runner)
