from __future__ import annotations

import shlex
import time

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import _resource_plateau as plateau
from tests.host.linux import _resource_plateau_gate as gate
from tests.host.linux.flows import tunnel_b_to_a_fixture as fixture


def add_second_endpoint(env: dict) -> None:
    credential = env["server_runner"](
        "server",
        "user",
        "add",
        "--json",
        "--path",
        env["server_install_path"],
        "--config-dir",
        helpers.SERVER_CONFIG_DIR_NAME,
        "--id",
        "resource-plateau-second-endpoint@example.com",
        "--host",
        gate.SECOND_ENDPOINT_IP,
        check=True,
    )
    link = helpers.parse_json_credential(credential.stdout or "")["link"]
    env["client_runner"](
        "client",
        "install",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
        "--mode",
        "proxy",
        "--link",
        link,
        "--force",
        check=True,
    )
    endpoints = helpers.read_pending_client_config(env["client_host"]).get("endpoints") or []
    hosts = {item.get("hostname") for item in endpoints}
    assert {fixture.SERVER_IP, gate.SECOND_ENDPOINT_IP} <= hosts


def assert_non_200(env: dict, _index: int) -> None:
    url = f"https://{fixture.SERVER_IP}:{fixture.SERVER_DIAGNOSTICS_PORT}/subscription"
    result = env["client_host"].run(
        "curl --silent --show-error --insecure --output /dev/null "
        f"--write-out '%{{http_code}}' {shlex.quote(url)}"
    )
    status = (result.stdout or "").strip()
    if result.rc != 0 or not status.startswith(("4", "5")):
        pytest.fail(f"expected non-200 control response, got rc={result.rc}, status={status}")


def heartbeat_attempts(host) -> dict[str, int]:
    state = helpers.read_json(host, helpers.CLIENT_HEARTBEAT_STATE_FILE)
    return {
        key: int(entry.get("attempts") or 0)
        for key, entry in (state.get("entries") or {}).items()
    }


def wait_for_recovery(env: dict, aux_host, baselines: dict) -> None:
    deadline = time.monotonic() + 75.0
    while time.monotonic() < deadline:
        recovered = True
        details = {}
        for name, host in (("client", env["client_host"]), ("aux", aux_host)):
            state = helpers.read_json(host, helpers.CLIENT_HEARTBEAT_STATE_FILE)
            entries = state.get("entries") or {}
            details[name] = entries
            baseline = baselines[name]
            recovered = recovered and bool(entries) and all(
                entry.get("status") == "healthy"
                and int(entry.get("attempts") or 0) > baseline.get(key, 0)
                for key, entry in entries.items()
            )
        if recovered:
            fixture.verify_heartbeat_state(env)
            return
        time.sleep(0.5)
    pytest.fail(f"fresh healthy heartbeats did not recover before plateau measurement: {details}")


def start_fd_leak(host) -> int:
    command = r"""nohup python3 -c '
import os
import time
files = []
while True:
    files.append(open("/dev/null"))
    time.sleep(0.05)
' >/tmp/xp2p-resource-leak.log 2>&1 & echo $!"""
    result = host.run(command)
    if result.rc != 0 or not (result.stdout or "").strip():
        pytest.fail(f"failed to start resource leak fixture: {result.stderr}")
    return int(result.stdout.strip())
