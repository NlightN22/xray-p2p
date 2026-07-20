from __future__ import annotations

import json
import os
import time
from pathlib import Path

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import env as linux_env

try:
    import tomllib
except ImportError:  # pragma: no cover
    import tomli as tomllib


FIXTURE_ROOT = Path(__file__).parent / "fixtures" / "upgrade_recovery"
APPLY_REQUEST = helpers.STATE_ROOT / "apply.request"
APPLY_ERROR = helpers.STATE_ROOT / "apply.error"
RUNTIME_META = helpers.CLIENT_LIVE_DIR / "runtime.json"
SERVICE_TIMEOUT = 60.0

SKIP_SERVICE_CLI = os.environ.get("XP2P_RUN_SERVICE_CLI_TESTS", "").strip().lower() not in {
    "1",
    "true",
    "yes",
}

pytestmark = [
    pytest.mark.host,
    pytest.mark.linux,
    pytest.mark.skipif(SKIP_SERVICE_CLI, reason="service CLI tests are opt-in"),
]


def _fixture_text(name: str) -> str:
    return (FIXTURE_ROOT / name).read_text(encoding="utf-8")


def _wait_for_recovery(host, runner) -> dict:
    deadline = time.time() + SERVICE_TIMEOUT
    last_runtime: dict = {}
    while time.time() < deadline:
        status = runner("client", "service", "status")
        if linux_env.path_exists(host, RUNTIME_META):
            last_runtime = helpers.read_json(host, RUNTIME_META)
        recovered = (
            status.rc == 0
            and last_runtime.get("version") != "0.2.6"
            and bool(last_runtime.get("control"))
            and not linux_env.path_exists(host, APPLY_REQUEST)
            and not linux_env.path_exists(host, APPLY_ERROR)
        )
        if recovered:
            return last_runtime
        time.sleep(1.0)
    raise AssertionError(f"Client did not recover stale apply state. Last runtime: {last_runtime}")


def _configure_fixture_hosts(host) -> None:
    lines = (
        "192.0.2.10 edge-a.example.test # xp2p-upgrade-recovery\n"
        "192.0.2.11 edge-b.example.test # xp2p-upgrade-recovery\n"
        "192.0.2.12 edge-c.example.test # xp2p-upgrade-recovery\n"
    )
    result = host.run(
        "printf %s '" + lines + "' | sudo -n tee -a /etc/hosts >/dev/null"
    )
    assert result.rc == 0, result.stderr


def _remove_fixture_hosts(host) -> None:
    host.run("sudo -n sed -i '/# xp2p-upgrade-recovery$/d' /etc/hosts")


def test_linux_service_recovers_real_shape_legacy_runtime(
    client_host, xp2p_client_runner
):
    client_text = _fixture_text("xp2p-client.toml")
    server = tomllib.loads(_fixture_text("xp2p-server.toml"))["server"]
    client = tomllib.loads(client_text)["client"]
    incident_user = server["users"][0]
    incident_endpoint = client["endpoints"][0]

    assert incident_endpoint["password"] == incident_user["previous_credential_for_rotation"]
    assert incident_endpoint["password"] != incident_user["active_credential"]

    try:
        _configure_fixture_hosts(client_host)
        xp2p_client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "192.0.2.10",
            "--user",
            "site-client",
            "--password",
            "legacy-credential-site-a",
            "--force",
            check=True,
        )
        xp2p_client_runner("client", "service", "stop")

        mkdir = client_host.run(
            f"sudo -n install -d -m 0755 {helpers.CLIENT_LIVE_DIR}"
        )
        assert mkdir.rc == 0, mkdir.stderr
        helpers.write_text(client_host, helpers.CLIENT_CONFIG_FILE, client_text)
        helpers.write_text(
            client_host,
            RUNTIME_META,
            _fixture_text("client-runtime-v0.2.6.json"),
        )
        helpers.write_text(
            client_host, APPLY_REQUEST, _fixture_text("apply.request.json")
        )
        helpers.write_text(client_host, APPLY_ERROR, _fixture_text("apply.error.json"))

        xp2p_client_runner("client", "service", "start", check=True)
        runtime = _wait_for_recovery(client_host, xp2p_client_runner)

        assert len(runtime["desired"]["endpoints"]) == 3
        assert len(runtime["control"]["auth_users"]) == 3
        assert helpers.read_text(client_host, helpers.CLIENT_CONFIG_FILE) == client_text
    finally:
        xp2p_client_runner("client", "service", "stop")
        _remove_fixture_hosts(client_host)
