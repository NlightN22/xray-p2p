import json
from pathlib import Path

import pytest

from tests.host.win import env as _env
from tests.host.win.flows.render import render_desired_xray_json

CLIENT_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
CLIENT_CONFIG_DIR_NAME = "config-client"
CLIENT_CONFIG_DIR = _env.CONFIG_ROOT / CLIENT_CONFIG_DIR_NAME
CLIENT_RUN_LOG = Path(r"C:\xp2p\build\logs\win\xp2p-client-run.out")
CLIENT_CONFIG_FILE = _env.CONFIG_ROOT / "xp2p-client.toml"
CLIENT_APPLIED_STATE_FILE = _env.CONFIG_ROOT / "xp2p-client.state.json"
CLIENT_STATE_FILES = [
    CLIENT_CONFIG_FILE,
    CLIENT_APPLIED_STATE_FILE,
]
LINK_HOST = "link.example.test"
LINK_HOST_IP = "198.51.100.40"


def _cleanup_client_install(client_host, runner, msi_path: str) -> None:
    runner("client", "remove", "--all", "--ignore-missing")
    _env.cleanup_xp2p_install(
        client_host,
        config_dirs=[CLIENT_CONFIG_DIR],
        state_files=CLIENT_STATE_FILES,
        extra_paths=[CLIENT_RUN_LOG],
    )


def _remote_path_exists(client_host, path: Path) -> bool:
    resolved = _env.resolve_config_path(client_host, path)
    quoted = _env.ps_quote(str(resolved))
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


def _expand_pending_targets(paths: list[Path]) -> list[Path]:
    targets: list[Path] = []
    for path in paths:
        pending = _env.pending_candidate(path)
        targets.append(pending)
        if pending != path:
            targets.append(path)
    return targets


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


def _add_hosts_entry(host, ip: str, hostname: str) -> None:
    result = _env.run_guest_script(
        host,
        "scripts/update_hosts_entry.ps1",
        Action="Add",
        HostName=hostname,
        IPAddress=ip,
    )
    if result.rc != 0:
        pytest.fail(
            "Failed to add hosts entry.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def _remove_hosts_entry(host, hostname: str) -> None:
    _env.run_guest_script(
        host,
        "scripts/update_hosts_entry.ps1",
        Action="Remove",
        HostName=hostname,
    )


def _find_outbound(data: dict, tag: str) -> dict:
    for outbound in data.get("outbounds", []):
        if outbound.get("tag") == tag:
            return outbound
    raise AssertionError(f"Expected outbound with tag {tag} to exist")


def _assert_outbound_entry(
    data: dict,
    host: str,
    password: str,
    email: str,
    server_name: str,
    allow_insecure: bool = False,
    pinned_peer_sha256=None,
    verify_peer_name=None,
) -> None:
    tag = _expected_tag(host)
    outbound = _find_outbound(data, tag)
    server = outbound["settings"]["servers"][0]
    assert server["address"] == host
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


def _assert_outbound_entry_any_host(
    data: dict,
    hosts: set[str],
    password: str,
    email: str,
    server_name: str,
    allow_insecure: bool = False,
    pinned_peer_sha256=None,
    verify_peer_name=None,
) -> None:
    tag = _expected_tag(server_name)
    outbound = _find_outbound(data, tag)
    server = outbound["settings"]["servers"][0]
    actual_host = server.get("address")
    assert actual_host in hosts, f"Unexpected outbound address {actual_host!r}"
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


def _assert_routing_rule(data: dict, host: str) -> None:
    rules = data.get("routing", {}).get("rules", [])
    for rule in rules:
        if rule.get("outboundTag") == "direct" and host in rule.get("ip", []):
            return
    raise AssertionError(f"Expected routing rule for {host} -> direct")


@pytest.mark.host
@pytest.mark.win
def test_client_install_and_force_overwrites(client_host, xp2p_client_runner, xp2p_msi_path):
    _cleanup_client_install(client_host, xp2p_client_runner, xp2p_msi_path)
    try:
        xp2p_client_runner(
            "client",
            "install",
            "--host",
            "10.62.10.10",
            "--user",
            "alpha@example.com",
            "--password",
            "test_password123",
            check=True,
            )

        data = render_desired_xray_json(xp2p_client_runner, role="client")
        _assert_outbound_entry(data, "10.62.10.10", "test_password123", "alpha@example.com", "10.62.10.10")

        xp2p_client_runner(
            "client",
            "install",
            "--host",
            "10.62.10.11",
            "--user",
            "beta@example.com",
            "--password",
            "override_password456",
            "--sni",
            "vpn.example.local",
            check=True,
            )

        updated_outbounds = render_desired_xray_json(xp2p_client_runner, role="client")
        _assert_outbound_entry(
            updated_outbounds,
            "10.62.10.10",
            "test_password123",
            "alpha@example.com",
            "10.62.10.10",
            allow_insecure=False,
            )
        _assert_outbound_entry(
            updated_outbounds,
            "10.62.10.11",
            "override_password456",
            "beta@example.com",
            "vpn.example.local",
            allow_insecure=False,
            )
        _assert_routing_rule(updated_outbounds, "10.62.10.10")
        _assert_routing_rule(updated_outbounds, "10.62.10.11")

        state = _env.read_toml(client_host, CLIENT_CONFIG_FILE).get("client") or {}
        recorded_hosts = {entry["hostname"] for entry in state.get("endpoints", [])}
        assert recorded_hosts == {"10.62.10.10", "10.62.10.11"}

        duplicate = xp2p_client_runner(
            "client",
            "install",
            "--host",
            "10.62.10.10",
            "--user",
            "gamma@example.com",
            "--password",
            "new-password",
            check=False,
            )
        assert duplicate.rc != 0, "Expected duplicate endpoint install to fail without --force"
        combined = f"{duplicate.stdout}\n{duplicate.stderr}".lower()
        assert "endpoint 10.62.10.10:8443 already exists" in combined

        xp2p_client_runner(
            "client",
            "install",
            "--host",
            "10.62.10.10",
            "--user",
            "gamma@example.com",
            "--password",
            "force-password",
            "--sni",
            "override.example",
            "--force",
            check=True,
            )

        refreshed = render_desired_xray_json(xp2p_client_runner, role="client")
        _assert_outbound_entry(
            refreshed,
            "10.62.10.10",
            "force-password",
            "gamma@example.com",
            "override.example",
            allow_insecure=False,
            )
        _assert_outbound_entry(
            refreshed,
            "10.62.10.11",
            "override_password456",
            "beta@example.com",
            "vpn.example.local",
            allow_insecure=False,
            )
    finally:
        _cleanup_client_install(client_host, xp2p_client_runner, xp2p_msi_path)


@pytest.mark.host
@pytest.mark.win
def test_client_install_from_link(client_host, xp2p_client_runner, xp2p_msi_path):
    _cleanup_client_install(client_host, xp2p_client_runner, xp2p_msi_path)
    host_entry_added = False
    try:
        _add_hosts_entry(client_host, LINK_HOST_IP, LINK_HOST)
        host_entry_added = True
        link = (
            f"trojan://linkpass@{LINK_HOST}:58443?"
            f"pinnedPeerCertSha256=deadbeef&security=tls&sni={LINK_HOST}&"
            f"verifyPeerCertByName={LINK_HOST}#link@example.com"
        )
        xp2p_client_runner(
            "client",
            "install",
            "--link",
            link,
            "--force",
            check=True,
            )

        data = render_desired_xray_json(xp2p_client_runner, role="client")
        _assert_outbound_entry_any_host(
            data,
            {LINK_HOST, LINK_HOST_IP},
            "linkpass",
            "link@example.com",
            LINK_HOST,
            pinned_peer_sha256="deadbeef",
            verify_peer_name=LINK_HOST,
            )
    finally:
        if host_entry_added:
            _remove_hosts_entry(client_host, LINK_HOST)
        _cleanup_client_install(client_host, xp2p_client_runner, xp2p_msi_path)


@pytest.mark.host
@pytest.mark.win
def test_client_install_from_link_without_allow_insecure(client_host, xp2p_client_runner, xp2p_msi_path):
    _cleanup_client_install(client_host, xp2p_client_runner, xp2p_msi_path)
    host_entry_added = False
    try:
        _add_hosts_entry(client_host, LINK_HOST_IP, LINK_HOST)
        host_entry_added = True
        link = (
            f"trojan://linkpass@{LINK_HOST}:58443?"
            f"security=tls&sni={LINK_HOST}#link@example.com"
        )
        xp2p_client_runner(
            "client",
            "install",
            "--link",
            link,
            "--force",
            check=True,
            )

        data = render_desired_xray_json(xp2p_client_runner, role="client")
        _assert_outbound_entry_any_host(
            data, {LINK_HOST, LINK_HOST_IP}, "linkpass", "link@example.com", LINK_HOST, allow_insecure=False
        )
    finally:
        if host_entry_added:
            _remove_hosts_entry(client_host, LINK_HOST)
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
            "10.62.10.10",
            "--user",
            "gamma@example.com",
            "--password",
            "runtime_password789",
            "--force",
            check=True,
            )
        xp2p_client_runner(
            "client",
            "mode",
            "proxy",
            "--path",
            str(CLIENT_INSTALL_DIR),
            "--config-dir",
            CLIENT_CONFIG_DIR_NAME,
            check=True,
        )

        with xp2p_client_run_factory(
            str(CLIENT_INSTALL_DIR), CLIENT_CONFIG_DIR_NAME
        ) as session:
            assert session["pid"] > 0

        assert _remote_path_exists(client_host, CLIENT_RUN_LOG), (
            f"Expected log file {CLIENT_RUN_LOG} to be created"
        )
        log_content = _env.run_powershell(
            client_host,
            f"$ErrorActionPreference='Stop'; Get-Content -Raw {_env.ps_quote(str(CLIENT_RUN_LOG))}",
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
            "10.62.10.50",
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
            "10.62.10.50",
            "--user",
            "state2@example.com",
            "--password",
            "state-pass-2",
            check=False,
            )
        assert result.rc != 0, "Expected install to fail when endpoint exists without --force"
        combined = f"{result.stdout}\n{result.stderr}".strip().lower()
        assert "endpoint 10.62.10.50:8443 already exists" in combined

        xp2p_client_runner(
            "client",
            "install",
            "--host",
            "10.62.10.50",
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
            "10.62.10.60",
            "--user",
            "nostate@example.com",
            "--password",
            "nostate-pass",
            "--force",
            check=True,
            )

        targets = _expand_pending_targets(CLIENT_STATE_FILES)
        _env.remove_paths(client_host, targets)
        assert not _env.paths_exist(client_host, targets), (
            "Expected client state files to be removed before re-install"
        )

        xp2p_client_runner(
            "client",
            "install",
            "--host",
            "10.62.10.61",
            "--user",
            "nostate2@example.com",
            "--password",
            "nostate-pass-2",
            check=True,
            )

        existing = _env.paths_exist(client_host, CLIENT_STATE_FILES)
        assert str(CLIENT_CONFIG_FILE) in existing, (
            "Expected client config file to be recreated"
        )
    finally:
        _cleanup_client_install(client_host, xp2p_client_runner, xp2p_msi_path)
