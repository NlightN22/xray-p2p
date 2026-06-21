from __future__ import annotations

import json
import time

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import _runtime_disable as runtime
from tests.host.linux import env as linux_env

pytestmark = [
    pytest.mark.host,
    pytest.mark.linux,
    pytest.mark.serial,
]

CONTROL_PORT = "62022"
TROJAN_PORT = "62310"
SERVER_HOST = "subscription-control.example.com"
USER = "subscription-control@example.com"
PASSWORD = "550e8400-e29b-41d4-a716-446655440001"
LEGACY_USER = "forced-legacy@example.com"
LEGACY_PASSWORD = "forced-legacy-password"
UUID_USER = "uuid@example.com"
UUID_PASSWORD = "550e8400-e29b-41d4-a716-446655440000"
VLESS_USER = "vless-profile@example.com"
VLESS_PASSWORD = "550e8400-e29b-41d4-a716-446655440002"
PROFILE_SWITCH_USER = "profile-switch@example.com"
PROFILE_SWITCH_PASSWORD = "550e8400-e29b-41d4-a716-446655440003"


def test_subscription_control_plane_uses_tls_and_hmac(server_host):
    runner = runtime.xp2p_runner(server_host)
    try:
        runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--host",
            SERVER_HOST,
            "--port",
            TROJAN_PORT,
            "--force",
            check=True,
        )
        runner(
            "server",
            "user",
            "add",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--id",
            USER,
            "--password",
            PASSWORD,
            "--host",
            SERVER_HOST,
            check=True,
        )
        runtime.start_service(server_host, runner, "server")

        result = linux_env.run_guest_script(
            server_host,
            "scripts/linux/check_subscription_control_plane.sh",
            "127.0.0.1",
            CONTROL_PORT,
            USER,
            PASSWORD,
            SERVER_HOST,
            TROJAN_PORT,
            timeout=60,
        )
        if result.rc != 0:
            helpers.dump_failure_state(server_host, "subscription-control-plane")
            pytest.fail(
                "subscription control probe failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        output = json.loads(result.stdout)
        assert output.get("generation"), f"probe did not report generation: {output}"
        assert output.get("heartbeat_tag") == "subscription-control"

        state = helpers.read_json(server_host, helpers.SERVER_HEARTBEAT_STATE_FILE)
        entries = state.get("entries") or {}
        assert any(entry.get("tag") == "subscription-control" for entry in entries.values())

        runner("server", "user", "rotate", USER, check=True)
        result = linux_env.run_guest_script(
            server_host,
            "scripts/linux/check_credential_rotation.sh",
            "127.0.0.1",
            CONTROL_PORT,
            USER,
            PASSWORD,
            timeout=60,
        )
        if result.rc != 0:
            helpers.dump_failure_state(server_host, "credential-rotation")
            pytest.fail(
                "credential rotation probe failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        rotation = json.loads(result.stdout)
        assert rotation.get("credential_generation") == 2
        assert rotation.get("subscription_generation")
    finally:
        runtime.stop_service(runner, "server")


def test_service_start_forces_legacy_credential_rotation(server_host):
    runner = runtime.xp2p_runner(server_host)
    try:
        runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--host",
            SERVER_HOST,
            "--port",
            TROJAN_PORT,
            "--force",
            check=True,
        )
        for user, password in ((LEGACY_USER, LEGACY_PASSWORD), (UUID_USER, UUID_PASSWORD)):
            runner(
                "server",
                "user",
                "add",
                "--path",
                helpers.INSTALL_ROOT.as_posix(),
                "--config-dir",
                helpers.SERVER_CONFIG_DIR_NAME,
                "--id",
                user,
                "--password",
                password,
                "--host",
                SERVER_HOST,
                check=True,
            )

        runtime.start_service(server_host, runner, "server")
        live = runtime.wait_for_live_xray(server_host, "server")
        credentials = _trojan_credentials(live)
        legacy_active = credentials.get(LEGACY_USER, "")
        assert legacy_active and legacy_active != LEGACY_PASSWORD
        assert _is_uuid(legacy_active)
        assert credentials.get(UUID_USER) == UUID_PASSWORD

        result = linux_env.run_guest_script(
            server_host,
            "scripts/linux/check_credential_rotation.sh",
            "127.0.0.1",
            CONTROL_PORT,
            LEGACY_USER,
            LEGACY_PASSWORD,
            timeout=60,
        )
        if result.rc != 0:
            helpers.dump_failure_state(server_host, "forced-credential-rotation")
            pytest.fail(
                "forced credential rotation probe failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        rotation = json.loads(result.stdout)
        assert rotation.get("credential_generation") == 2
        assert rotation.get("subscription_generation")

        desired = helpers.read_pending_server_config(server_host)
        users = {entry["user_label"]: entry for entry in desired.get("users") or []}
        assert users[LEGACY_USER]["credential_generation"] == 2
        assert users[LEGACY_USER]["active_credential"] == legacy_active
        assert "previous_credential_for_rotation" not in users[LEGACY_USER]
        assert users[UUID_USER]["active_credential"] == UUID_PASSWORD
        assert users[UUID_USER]["credential_generation"] == 1
    finally:
        runtime.stop_service(runner, "server")


def test_subscription_control_plane_publishes_vless_tls_vision_profile(server_host):
    runner = runtime.xp2p_runner(server_host)
    try:
        runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--host",
            SERVER_HOST,
            "--port",
            TROJAN_PORT,
            "--force",
            check=True,
        )
        runner("server", "profile", "vless-tls-vision", check=True)
        runner(
            "server",
            "user",
            "add",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--id",
            VLESS_USER,
            "--password",
            VLESS_PASSWORD,
            "--host",
            SERVER_HOST,
            check=True,
        )

        runtime.start_service(server_host, runner, "server")
        live = runtime.wait_for_live_xray(server_host, "server")
        users = _vless_users(live)
        assert users[VLESS_USER]["id"] == VLESS_PASSWORD
        assert users[VLESS_USER]["flow"] == "xtls-rprx-vision"

        result = linux_env.run_guest_script(
            server_host,
            "scripts/linux/check_subscription_control_plane.sh",
            "127.0.0.1",
            CONTROL_PORT,
            VLESS_USER,
            VLESS_PASSWORD,
            SERVER_HOST,
            TROJAN_PORT,
            "vless-tls-vision",
            "vless",
            "xtls-rprx-vision",
            timeout=60,
        )
        if result.rc != 0:
            helpers.dump_failure_state(server_host, "subscription-control-plane-vless")
            pytest.fail(
                "VLESS subscription control probe failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        output = json.loads(result.stdout)
        assert output.get("profile") == "vless-tls-vision"
    finally:
        runtime.stop_service(runner, "server")


def test_client_switches_profiles_bidirectionally_from_subscription(client_host, server_host):
    server_runner = runtime.xp2p_runner(server_host)
    client_runner = runtime.xp2p_runner(client_host)
    server_ip = _detect_host_ipv4(server_host)
    try:
        _add_hosts_entry(client_host, server_ip, SERVER_HOST)
        server_runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--host",
            SERVER_HOST,
            "--port",
            TROJAN_PORT,
            "--force",
            check=True,
        )
        user_add = server_runner(
            "server",
            "user",
            "add",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--id",
            PROFILE_SWITCH_USER,
            "--password",
            PROFILE_SWITCH_PASSWORD,
            "--host",
            SERVER_HOST,
            check=True,
        )
        link = _extract_link(user_add.stdout or "")
        client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--link",
            link,
            "--mode",
            "proxy",
            check=True,
        )

        runtime.start_service(server_host, server_runner, "server")
        runtime.start_service(client_host, client_runner, "client")
        assert _client_outbound_protocol(runtime.wait_for_live_xray(client_host, "client"), SERVER_HOST) == "trojan"
        _assert_client_profile(client_host, SERVER_HOST, "trojan-tls", "trojan", "")
        _assert_tunnel_ping(client_host, server_host, client_runner, SERVER_HOST)
        client_apply_count = _wait_for_client_subscription_apply_count(client_host, 0)
        _assert_client_subscription_apply_count_stable(client_host, client_apply_count, duration=35.0)

        server_runner("server", "profile", "vless-tls-vision", check=True)
        server_live = runtime.wait_for_live_xray(server_host, "server")
        assert PROFILE_SWITCH_USER in _vless_users(server_live)
        _wait_for_client_profile(client_host, SERVER_HOST, server_ip, "vless-tls-vision", "vless", "xtls-rprx-vision")
        client_apply_count = _wait_for_client_subscription_apply_count(client_host, client_apply_count + 1)
        _assert_client_subscription_apply_count_stable(client_host, client_apply_count)
        endpoint = _client_endpoint_any(client_host, SERVER_HOST, server_ip)
        live = runtime.wait_for_live_xray(client_host, "client")
        assert _client_outbound_protocol_by_tag(live, endpoint["tag"]) == "vless"
        _assert_tunnel_ping(client_host, server_host, client_runner, SERVER_HOST)

        server_runner("server", "profile", "trojan-tls", check=True)
        server_live = runtime.wait_for_live_xray(server_host, "server")
        assert PROFILE_SWITCH_USER in _trojan_credentials(server_live)
        _wait_for_client_profile(client_host, SERVER_HOST, server_ip, "trojan-tls", "trojan", "")
        client_apply_count = _wait_for_client_subscription_apply_count(client_host, client_apply_count + 1)
        _assert_client_subscription_apply_count_stable(client_host, client_apply_count)
        endpoint = _client_endpoint_any(client_host, SERVER_HOST, server_ip)
        live = runtime.wait_for_live_xray(client_host, "client")
        assert _client_outbound_protocol_by_tag(live, endpoint["tag"]) == "trojan"
        _assert_tunnel_ping(client_host, server_host, client_runner, SERVER_HOST)
    finally:
        runtime.stop_service(client_runner, "client")
        runtime.stop_service(server_runner, "server")


def _trojan_credentials(xray: dict) -> dict[str, str]:
    for inbound in xray.get("inbounds") or []:
        if inbound.get("protocol") == "trojan":
            return {
                str(client.get("email")): str(client.get("password"))
                for client in inbound.get("settings", {}).get("clients") or []
                if client.get("email") and client.get("password")
            }
    raise AssertionError("Trojan inbound is missing")


def _extract_link(output: str) -> str:
    for raw in (output or "").splitlines():
        stripped = raw.strip()
        for scheme in ("trojan://", "vless://"):
            index = stripped.find(scheme)
            if index >= 0:
                return stripped[index:]
    pytest.fail(f"xp2p server user add did not emit a connection link.\nSTDOUT:\n{output}")


def _vless_users(xray: dict) -> dict[str, dict]:
    for inbound in xray.get("inbounds") or []:
        if inbound.get("protocol") == "vless":
            return {
                str(client.get("email")): client
                for client in inbound.get("settings", {}).get("clients") or []
                if client.get("email")
            }
    raise AssertionError("VLESS inbound is missing")


def _client_outbound_protocol(xray: dict, host: str) -> str:
    tag = helpers.expected_proxy_tag(host)
    return _client_outbound_protocol_by_tag(xray, tag)


def _client_outbound_protocol_by_tag(xray: dict, tag: str) -> str:
    for outbound in xray.get("outbounds") or []:
        if outbound.get("tag") == tag:
            return str(outbound.get("protocol") or "")
    raise AssertionError(f"Client outbound {tag} is missing")


def _assert_client_profile(host, endpoint_host: str, profile: str, protocol: str, flow: str) -> None:
    endpoint = _client_endpoint(host, endpoint_host)
    assert endpoint.get("profile") == profile
    assert endpoint.get("protocol") == protocol
    if flow:
        assert endpoint.get("flow") == flow


def _wait_for_client_profile(host, endpoint_host: str, fallback_host: str, profile: str, protocol: str, flow: str) -> None:
    deadline = time.time() + 120.0
    last_state = {}
    while time.time() < deadline:
        try:
            endpoint = _client_endpoint_any(host, endpoint_host, fallback_host)
            assert endpoint.get("profile") == profile
            assert endpoint.get("protocol") == protocol
            if flow:
                assert endpoint.get("flow") == flow
            live = runtime.wait_for_live_xray(host, "client")
            if _client_outbound_protocol(live, endpoint_host) == protocol:
                return
        except (AssertionError, RuntimeError) as exc:
            last_state = {"error": str(exc)}
        time.sleep(2.0)
    helpers.dump_failure_state(host, "client-profile-subscription-switch")
    raise AssertionError(f"Client did not switch to {profile}: {last_state}")


def _client_endpoint(host, endpoint_host: str) -> dict:
    state = helpers.read_pending_client_config(host)
    for endpoint in state.get("endpoints") or []:
        if endpoint.get("hostname") == endpoint_host:
            return endpoint
    raise AssertionError(f"Client endpoint {endpoint_host} is missing: {state.get('endpoints')}")


def _client_endpoint_any(host, *endpoint_hosts: str) -> dict:
    state = helpers.read_pending_client_config(host)
    expected = {value for value in endpoint_hosts if value}
    for endpoint in state.get("endpoints") or []:
        if endpoint.get("hostname") in expected:
            return endpoint
    raise AssertionError(f"Client endpoint {sorted(expected)} is missing: {state.get('endpoints')}")


def _assert_tunnel_ping(client_host, server_host, runner, host: str) -> None:
    deadline = time.time() + 45.0
    last = None
    while time.time() < deadline:
        last = runner("ping", host, "-T", "--count", "2", check=False)
        output = ((last.stdout or "") + (last.stderr or "")).lower()
        if last.rc == 0 and "0% loss" in output:
            return
        time.sleep(2.0)
    helpers.dump_failure_state(client_host, "client-profile-switch-tunnel-ping")
    helpers.dump_failure_state(server_host, "server-profile-switch-tunnel-ping")
    raise AssertionError(f"Tunnel ping failed after profile switch.\nSTDOUT:\n{last.stdout}\nSTDERR:\n{last.stderr}")


def _wait_for_client_subscription_apply_count(host, expected: int, *, timeout: float = 45.0) -> int:
    deadline = time.time() + timeout
    last_counts = (0, 0)
    while time.time() < deadline:
        last_counts = _client_subscription_apply_counts(host)
        if last_counts[0] == expected and last_counts[1] <= expected:
            return expected
        if last_counts[0] > expected or last_counts[1] > expected:
            _fail_unexpected_subscription_apply_count(host, expected, last_counts)
        time.sleep(1.0)
    _fail_unexpected_subscription_apply_count(host, expected, last_counts)
    return expected


def _assert_client_subscription_apply_count_stable(host, expected: int, *, duration: float = 8.0) -> None:
    deadline = time.time() + duration
    while time.time() < deadline:
        counts = _client_subscription_apply_counts(host)
        if counts[0] != expected or counts[1] > expected:
            _fail_unexpected_subscription_apply_count(host, expected, counts)
        time.sleep(1.0)


def _client_subscription_apply_counts(host) -> tuple[int, int]:
    log = _read_client_service_log(host)
    return (
        log.count("subscription applied."),
        log.count("runtime outbound apply completed. role: client"),
    )


def _read_client_service_log(host) -> str:
    path = helpers.LOG_ROOT / "client" / "service.log"
    if not helpers.path_exists(host, path):
        return ""
    return helpers.read_text(host, path)


def _fail_unexpected_subscription_apply_count(host, expected: int, counts: tuple[int, int]) -> None:
    helpers.dump_failure_state(host, "client-subscription-apply-count")
    log_tail = "\n".join(_read_client_service_log(host).splitlines()[-80:])
    raise AssertionError(
        "Client subscription apply count changed without a matching server-side subscription update.\n"
        f"Expected subscription count: {expected}; runtime apply count must not exceed it.\n"
        f"Actual subscription/runtime counts: {counts[0]}/{counts[1]}\n"
        f"Client service log tail:\n{log_tail}"
    )


def _add_hosts_entry(host, ip_address: str, name: str) -> None:
    escaped_name = name.replace("'", "'\\''")
    escaped_ip = ip_address.replace("'", "'\\''")
    host.run(
        "sudo -n /bin/sh -c "
        f"'tmp=$(mktemp); grep -v \"[[:space:]]{escaped_name}$\" /etc/hosts > \"$tmp\"; "
        f"printf \"%s %s\\n\" \"{escaped_ip}\" \"{escaped_name}\" >> \"$tmp\"; "
        "cat \"$tmp\" > /etc/hosts; rm -f \"$tmp\"'"
    )


def _detect_host_ipv4(host) -> str:
    result = host.run("ip -o -4 addr show scope global | awk '{print $4}' | cut -d/ -f1")
    if result.rc != 0:
        pytest.fail(f"Failed to detect host IPv4.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}")
    addresses = [line.strip() for line in result.stdout.splitlines() if line.strip()]
    if not addresses:
        pytest.fail("Failed to detect host IPv4: no global addresses found")
    for address in addresses:
        if not address.startswith("10.0.2."):
            return address
    return addresses[0]


def _is_uuid(value: str) -> bool:
    parts = value.split("-")
    return [len(part) for part in parts] == [8, 4, 4, 4, 12] and all(
        all(char in "0123456789abcdef" for char in part.lower()) for part in parts
    )
