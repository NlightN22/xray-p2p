from __future__ import annotations

import json

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


def _trojan_credentials(xray: dict) -> dict[str, str]:
    for inbound in xray.get("inbounds") or []:
        if inbound.get("protocol") == "trojan":
            return {
                str(client.get("email")): str(client.get("password"))
                for client in inbound.get("settings", {}).get("clients") or []
                if client.get("email") and client.get("password")
            }
    raise AssertionError("Trojan inbound is missing")


def _is_uuid(value: str) -> bool:
    parts = value.split("-")
    return [len(part) for part in parts] == [8, 4, 4, 4, 12] and all(
        all(char in "0123456789abcdef" for char in part.lower()) for part in parts
    )
