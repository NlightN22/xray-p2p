import json
from pathlib import Path

import pytest

from tests.host.win import env as _env

CLIENT_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
CLIENT_CONFIG_DIR_NAME = "config-client"
CLIENT_CONFIG_DIR = CLIENT_INSTALL_DIR / CLIENT_CONFIG_DIR_NAME
CLIENT_CONFIG_OUTBOUNDS = CLIENT_CONFIG_DIR / "outbounds.json"
CLIENT_ROUTING_JSON = CLIENT_CONFIG_DIR / "routing.json"
CLIENT_LOG_RELATIVE = r"logs\client.err"
CLIENT_LOG_FILE = CLIENT_INSTALL_DIR / CLIENT_LOG_RELATIVE
CLIENT_STATE_FILES = [
    CLIENT_INSTALL_DIR / "install-state-client.json",
    CLIENT_INSTALL_DIR / "install-state.json",
]
CLIENT_STATE_FILE = CLIENT_STATE_FILES[0]


def _cleanup_client_install(client_host, runner, msi_path: str) -> None:
    runner("client", "remove", "--all", "--ignore-missing")
    _env.install_xp2p_from_msi(client_host, msi_path)


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


def _expected_tag(host: str) -> str:
    cleaned = host.strip().lower()
    result = []
    last_dash = False
    for char in cleaned:
        if char.isalnum():
            result.append(char)
            last_dash = False
            continue
        if char == "-":
            result.append(char)
            last_dash = False
            continue
        if not last_dash:
            result.append("-")
            last_dash = True
    sanitized = "".join(result).strip("-")
    if not sanitized:
        sanitized = "endpoint"
    return f"proxy-{sanitized}"


def _find_outbound(data: dict, tag: str) -> dict:
    for outbound in data.get("outbounds", []):
        if outbound.get("tag") == tag:
            return outbound
    raise AssertionError(f"Expected outbound with tag {tag} to exist")


def _assert_outbound_entry(
    data: dict, host: str, password: str, email: str, server_name: str, allow_insecure: bool = False
) -> None:
    tag = _expected_tag(host)
    outbound = _find_outbound(data, tag)
    server = outbound["settings"]["servers"][0]
    assert server["address"] == host
    assert server["password"] == password
    assert server["email"] == email
    tls_settings = outbound["streamSettings"]["tlsSettings"]
    assert tls_settings["serverName"] == server_name
    assert bool(tls_settings.get("allowInsecure")) is bool(allow_insecure)


def _assert_routing_rule(data: dict, host: str) -> None:
    tag = _expected_tag(host)
    rules = data.get("routing", {}).get("rules", [])
    for rule in rules:
        if rule.get("outboundTag") == tag and host in rule.get("ip", []):
            return
    raise AssertionError(f"Expected routing rule for {host} -> {tag}")


@pytest.mark.host
@pytest.mark.win
def test_client_install_and_force_overwrites(client_host, xp2p_client_runner, xp2p_msi_path):
    _cleanup_client_install(client_host, xp2p_client_runner, xp2p_msi_path)
    try:
        xp2p_client_runner(
            "client",
            "install",
            "--host",
            "10.0.10.10",
            "--user",
            "alpha@example.com",
            "--password",
            "test_password123",
            check=True,
        )

        data = _read_remote_json(client_host, CLIENT_CONFIG_OUTBOUNDS)
        _assert_outbound_entry(data, "10.0.10.10", "test_password123", "alpha@example.com", "10.0.10.10")

        xp2p_client_runner(
            "client",
            "install",
            "--host",
            "10.0.10.11",
            "--user",
            "beta@example.com",
            "--password",
            "override_password456",
            "--server-name",
            "vpn.example.local",
            check=True,
        )

        updated_outbounds = _read_remote_json(client_host, CLIENT_CONFIG_OUTBOUNDS)
        _assert_outbound_entry(updated_outbounds, "10.0.10.10", "test_password123", "alpha@example.com", "10.0.10.10", allow_insecure=True)
        _assert_outbound_entry(
            updated_outbounds, "10.0.10.11", "override_password456", "beta@example.com", "vpn.example.local", allow_insecure=True
        )

        routing = _read_remote_json(client_host, CLIENT_ROUTING_JSON)
        _assert_routing_rule(routing, "10.0.10.10")
        _assert_routing_rule(routing, "10.0.10.11")

        state = _read_remote_json(client_host, CLIENT_STATE_FILE)
        recorded_hosts = {entry["hostname"] for entry in state.get("endpoints", [])}
        assert recorded_hosts == {"10.0.10.10", "10.0.10.11"}

        duplicate = xp2p_client_runner(
            "client",
            "install",
            "--host",
            "10.0.10.10",
            "--user",
            "gamma@example.com",
            "--password",
            "new-password",
            check=False,
        )
        assert duplicate.rc != 0, "Expected duplicate endpoint install to fail without --force"
        combined = f"{duplicate.stdout}\n{duplicate.stderr}".lower()
        assert "endpoint 10.0.10.10 already exists" in combined

        xp2p_client_runner(
            "client",
            "install",
            "--host",
            "10.0.10.10",
            "--user",
            "gamma@example.com",
            "--password",
            "force-password",
            "--server-name",
            "override.example",
            "--force",
            check=True,
        )

        refreshed = _read_remote_json(client_host, CLIENT_CONFIG_OUTBOUNDS)
        _assert_outbound_entry(refreshed, "10.0.10.10", "force-password", "gamma@example.com", "override.example", allow_insecure=True)
        _assert_outbound_entry(refreshed, "10.0.10.11", "override_password456", "beta@example.com", "vpn.example.local", allow_insecure=True)
    finally:
        _cleanup_client_install(client_host, xp2p_client_runner, xp2p_msi_path)


@pytest.mark.host
@pytest.mark.win
def test_client_install_from_link(client_host, xp2p_client_runner, xp2p_msi_path):
    _cleanup_client_install(client_host, xp2p_client_runner, xp2p_msi_path)
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
        _assert_outbound_entry(
            data, "link.example.test", "linkpass", "link@example.com", "link.example.test", allow_insecure=True
        )
    finally:
        _cleanup_client_install(client_host, xp2p_client_runner, xp2p_msi_path)


@pytest.mark.host
@pytest.mark.win
def test_client_run_starts_xray_core(
    client_host, xp2p_client_runner, xp2p_client_run_factory, xp2p_msi_path
):
    _cleanup_client_install(client_host, xp2p_client_runner, xp2p_msi_path)
    try:
        xp2p_client_runner(
            "client",
            "install",
            "--host",
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
        _cleanup_client_install(client_host, xp2p_client_runner, xp2p_msi_path)


@pytest.mark.host
@pytest.mark.win
def test_client_install_requires_force_for_existing_endpoint(
    client_host, xp2p_client_runner, xp2p_msi_path
):
    _cleanup_client_install(client_host, xp2p_client_runner, xp2p_msi_path)
    try:
        xp2p_client_runner(
            "client",
            "install",
            "--host",
            "10.0.10.50",
            "--user",
            "state@example.com",
            "--password",
            "state-pass",
            check=True,
        )

        result = xp2p_client_runner(
            "client",
            "install",
            "--host",
            "10.0.10.50",
            "--user",
            "state2@example.com",
            "--password",
            "state-pass-2",
            check=False,
        )
        assert result.rc != 0, "Expected install to fail when endpoint exists without --force"
        combined = f"{result.stdout}\n{result.stderr}".strip().lower()
        assert "endpoint 10.0.10.50 already exists" in combined

        xp2p_client_runner(
            "client",
            "install",
            "--host",
            "10.0.10.50",
            "--user",
            "state2@example.com",
            "--password",
            "state-pass-2",
            "--force",
            check=True,
        )
    finally:
        _cleanup_client_install(client_host, xp2p_client_runner, xp2p_msi_path)


@pytest.mark.host
@pytest.mark.win
def test_client_install_succeeds_without_state_marker(
    client_host, xp2p_client_runner, xp2p_msi_path
):
    _cleanup_client_install(client_host, xp2p_client_runner, xp2p_msi_path)
    try:
        xp2p_client_runner(
            "client",
            "install",
            "--host",
            "10.0.10.60",
            "--user",
            "nostate@example.com",
            "--password",
            "nostate-pass",
            "--force",
            check=True,
        )

        for state_file in CLIENT_STATE_FILES:
            _remove_remote_path(client_host, state_file)
        assert all(
            not _remote_path_exists(client_host, path) for path in CLIENT_STATE_FILES
        ), "Expected client state files to be removed before re-install"

        xp2p_client_runner(
            "client",
            "install",
            "--host",
            "10.0.10.61",
            "--user",
            "nostate2@example.com",
            "--password",
            "nostate-pass-2",
            check=True,
        )

        assert any(
            _remote_path_exists(client_host, path) for path in CLIENT_STATE_FILES
        ), "Expected client install-state file to be recreated"
    finally:
        _cleanup_client_install(client_host, xp2p_client_runner, xp2p_msi_path)
