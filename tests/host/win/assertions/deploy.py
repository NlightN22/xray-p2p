from __future__ import annotations

import time
from pathlib import Path

import pytest

from tests.host.win import env as win_env
from tests.host.win.diagnostics import remote_files

CLIENT_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
CLIENT_CONFIG_DIR_NAME = "config-client"
CLIENT_CONFIG_DIR = win_env.CONFIG_ROOT / CLIENT_CONFIG_DIR_NAME
CLIENT_CONFIG_FILE = win_env.CONFIG_ROOT / "xp2p-client.toml"
CLIENT_APPLIED_STATE_FILE = win_env.CONFIG_ROOT / "xp2p-client.state.json"
CLIENT_STATE_FILES = [
    CLIENT_CONFIG_FILE,
    CLIENT_APPLIED_STATE_FILE,
]

SERVER_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
SERVER_CONFIG_DIR_NAME = "config-server"
SERVER_CONFIG_DIR = win_env.CONFIG_ROOT / SERVER_CONFIG_DIR_NAME
SERVER_CERT_DEST = SERVER_CONFIG_DIR / "cert.pem"
SERVER_KEY_DEST = SERVER_CONFIG_DIR / "key.pem"
SERVER_CONFIG_FILE = win_env.CONFIG_ROOT / "xp2p-server.toml"
SERVER_STATE_FILE = win_env.CONFIG_ROOT / "xp2p-server.state.json"
SERVER_STATE_FILES = [
    SERVER_CONFIG_FILE,
    SERVER_STATE_FILE,
]

CLIENT_LIVE_XRAY_JSON = win_env.CONFIG_LIVE_ROOT / CLIENT_CONFIG_DIR_NAME / "xray.json"
SERVER_LIVE_XRAY_JSON = win_env.CONFIG_LIVE_ROOT / SERVER_CONFIG_DIR_NAME / "xray.json"

HEARTBEAT_STATE_FILES = [
    win_env.CONFIG_ROOT / "state-heartbeat.json",
    win_env.CONFIG_ROOT / "state-heartbeat-client.json",
    win_env.CONFIG_ROOT / "state-heartbeat-server.json",
]


def assert_client_install_artifacts(host, server_ip: str, user: str, password: str) -> None:
    missing = []
    for path in [
        CLIENT_INSTALL_DIR / "xp2p.exe",
        CLIENT_INSTALL_DIR / "bin" / "xray.exe",
        CLIENT_INSTALL_DIR / "bin" / "wintun.dll",
        CLIENT_LIVE_XRAY_JSON,
        CLIENT_CONFIG_FILE,
        CLIENT_APPLIED_STATE_FILE,
    ]:
        if not win_env.path_exists(host, path):
            missing.append(str(path))
    if missing:
        pytest.fail("Client deploy artifacts missing:\n" + "\n".join(missing))

    state = remote_files.read_remote_json(host, CLIENT_APPLIED_STATE_FILE)
    endpoints = state.get("config", {}).get("endpoints", [])
    expected_port = None
    if endpoints and isinstance(endpoints, list):
        expected_port = endpoints[0].get("port")
    xray = remote_files.read_remote_json(host, CLIENT_LIVE_XRAY_JSON)
    assert_outbound_entry(
        xray,
        server_ip,
        password,
        user,
        server_ip,
        pinned_peer_sha256="",
        verify_peer_name=server_ip,
        port=expected_port,
    )


def assert_client_state(host, server_ip: str) -> None:
    state = win_env.read_toml(host, CLIENT_CONFIG_FILE).get("client") or {}
    recorded_hosts = {entry.get("hostname") for entry in state.get("endpoints", [])}
    assert recorded_hosts == {server_ip}, f"Unexpected endpoint entries recorded: {recorded_hosts}"


def assert_client_routing(host, server_ip: str) -> None:
    xray = remote_files.read_remote_json(host, CLIENT_LIVE_XRAY_JSON)
    assert_routing_rule(xray, server_ip)


def assert_internet_access(host) -> None:
    script = """
$ErrorActionPreference = 'Stop'
$dnsName = "example.com"
$tcpHost = "1.1.1.1"
$tcpPort = 443
try {
    Resolve-DnsName -Name $dnsName -ErrorAction Stop | Out-Null
} catch {
    Write-Error "Internet check failed: DNS lookup for $dnsName"
    exit 1
}
try {
    $tcpOk = Test-NetConnection -ComputerName $tcpHost -Port $tcpPort -InformationLevel Quiet
} catch {
    $tcpOk = $false
}
if (-not $tcpOk) {
    Write-Error "Internet check failed: TCP connect to ${tcpHost}:${tcpPort}"
    exit 1
}
exit 0
"""
    result = win_env.run_powershell(host, script, label="read_optional_text")
    if result.rc != 0:
        pytest.fail(
            "Client internet check failed.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def expected_tag(host: str) -> str:
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


def find_trojan_inbound(data: dict) -> dict:
    for inbound in data.get("inbounds", []):
        if inbound.get("protocol") == "trojan":
            return inbound
    raise AssertionError("Expected trojan inbound in server configuration")


def normalize_windows_path(value: str | None) -> str:
    return (value or "").replace("\\", "/")


def assert_outbound_entry(
    data: dict,
    host: str,
    password: str,
    email: str,
    server_name: str,
    allow_insecure: bool = False,
    pinned_peer_sha256: str | None = None,
    verify_peer_name: str | None = None,
    port: int | None = None,
) -> None:
    tag = expected_tag(host)
    outbound = find_outbound(data, tag)
    server = outbound["settings"]["servers"][0]
    assert server["address"] == host
    if port is not None:
        assert server.get("port") == port
    assert server["password"] == password
    assert server["email"] == email
    tls_settings = outbound["streamSettings"]["tlsSettings"]
    assert tls_settings["serverName"] == server_name
    if pinned_peer_sha256 is not None:
        actual_pin = tls_settings.get("pinnedPeerCertSha256")
        if pinned_peer_sha256:
            assert actual_pin == pinned_peer_sha256
        else:
            assert actual_pin, "Expected pinnedPeerCertSha256 to be set"
        if verify_peer_name:
            assert tls_settings.get("verifyPeerCertByName") == verify_peer_name
        assert "allowInsecure" not in tls_settings or not tls_settings.get("allowInsecure")
    else:
        assert bool(tls_settings.get("allowInsecure")) is bool(allow_insecure)


def find_outbound(data: dict, tag: str) -> dict:
    for outbound in data.get("outbounds", []):
        if outbound.get("tag") == tag:
            return outbound
    raise AssertionError(f"Expected outbound with tag {tag} to exist")


def assert_routing_rule(data: dict, host: str) -> None:
    rules = data.get("routing", {}).get("rules", [])
    for rule in rules:
        if rule.get("outboundTag") == "direct" and host in rule.get("ip", []):
            return
    raise AssertionError(f"Expected routing rule for {host} -> direct")


def wait_for_heartbeat_state(host, *, timeout: int) -> dict:
    deadline = time.time() + timeout
    last_error: Exception | None = None
    while time.time() < deadline:
        for path in HEARTBEAT_STATE_FILES:
            if win_env.path_exists(host, path):
                try:
                    return remote_files.read_remote_json(host, path)
                except Exception as exc:  # noqa: BLE001
                    last_error = exc
        time.sleep(1)
    if last_error:
        raise AssertionError(f"Failed to read heartbeat state: {last_error}") from last_error
    raise AssertionError("Heartbeat state file not found on client host")


def assert_heartbeat_entry(
    state: dict,
    tag: str,
    *,
    host: str | None = None,
    user: str | None = None,
    client_ip: str | None = None,
) -> None:
    entries = state.get("entries")
    if not isinstance(entries, dict):
        raise AssertionError("Heartbeat state is missing entries map")
    normalized = (tag or "").strip().lower()
    if not normalized:
        raise AssertionError("Heartbeat tag to look up is empty")
    for entry in entries.values():
        entry_tag = (entry.get("tag") or "").strip()
        if entry_tag.lower() != normalized:
            continue
        if host is not None:
            recorded_host = (entry.get("host") or "").strip()
            if recorded_host != host.strip():
                raise AssertionError(
                    f"Heartbeat entry {entry_tag} host mismatch (expected {host}, got {recorded_host})"
                )
        if user is not None:
            recorded_user = (entry.get("user") or "").strip()
            if recorded_user != user.strip():
                raise AssertionError(
                    f"Heartbeat entry {entry_tag} user mismatch (expected {user}, got {recorded_user})"
                )
        if client_ip is not None:
            recorded_ip = (entry.get("client_ip") or "").strip()
            if recorded_ip != client_ip.strip():
                raise AssertionError(
                    f"Heartbeat entry {entry_tag} client IP mismatch (expected {client_ip}, got {recorded_ip})"
                )
        return
    raise AssertionError(f"Heartbeat entry for tag {tag} not found in state")
