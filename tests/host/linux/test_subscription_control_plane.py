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
PASSWORD = "subscription-control-secret"


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
