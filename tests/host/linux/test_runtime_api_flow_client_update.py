from __future__ import annotations

import json
import os

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import _runtime_disable as rt
from tests.host.linux import _runtime_disable_setup as setup

SKIP_RUNTIME_API_FLOW = os.environ.get("XP2P_RUN_SERVICE_CLI_TESTS", "").strip().lower() not in {
    "1",
    "true",
    "yes",
}
pytestmark = [
    pytest.mark.host,
    pytest.mark.linux,
    pytest.mark.skipif(SKIP_RUNTIME_API_FLOW, reason="runtime API flow service tests are opt-in"),
]

NEW_USER = "runtime-updated@example.com"
NEW_PASSWORD = "runtime-updated-pass"


def test_client_update_runtime_api_applies_without_restart(client_host):
    runner = rt.xp2p_runner(client_host)
    primary_tag, _, _ = setup.install_client_with_two_endpoints(runner)
    try:
        rt.start_service(client_host, runner, "client")
        before_pid = rt.wait_for_stable_xray_pid(client_host)

        runner("client", "update", primary_tag, "--user", NEW_USER, "--password", NEW_PASSWORD, check=True)

        rt.assert_same_xray_pid(client_host, before_pid, "client-update-runtime-pid-changed")
        desired = setup.client_desired(client_host)
        endpoint = setup.endpoint_by_tag(desired, primary_tag)
        assert endpoint.get("user") == NEW_USER
        assert endpoint.get("password") == NEW_PASSWORD
        live = rt.wait_for_live_xray(client_host, "client")
        helpers.assert_outbound(live, setup.CLIENT_PRIMARY, NEW_PASSWORD, NEW_USER, setup.CLIENT_PRIMARY)
        rt.assert_apply_clean(client_host)
    finally:
        rt.stop_service(runner, "client")


def test_client_update_stopped_service_stages_desired_only(client_host):
    runner = rt.xp2p_runner(client_host)
    primary_tag, _, _ = setup.install_client_with_two_endpoints(runner)
    try:
        rt.start_service(client_host, runner, "client")
        live_before = helpers.read_text(client_host, helpers.CLIENT_LIVE_DIR / "xray.json")
        rt.stop_service(runner, "client")
        rt.wait_for_service(client_host, "client", active=False)

        runner("client", "update", primary_tag, "--user", NEW_USER, "--password", NEW_PASSWORD, check=True)

        desired = setup.client_desired(client_host)
        endpoint = setup.endpoint_by_tag(desired, primary_tag)
        assert endpoint.get("user") == NEW_USER
        assert endpoint.get("password") == NEW_PASSWORD
        live_after = helpers.read_text(client_host, helpers.CLIENT_LIVE_DIR / "xray.json")
        assert live_after == live_before
        rt.assert_apply_clean(client_host)

        rt.start_service(client_host, runner, "client")
        live = rt.wait_for_live_xray(client_host, "client")
        helpers.assert_outbound(live, setup.CLIENT_PRIMARY, NEW_PASSWORD, NEW_USER, setup.CLIENT_PRIMARY)
    finally:
        rt.stop_service(runner, "client")


def test_client_update_running_service_fails_when_runtime_api_is_unavailable(client_host):
    runner = rt.xp2p_runner(client_host)
    primary_tag, _, _ = setup.install_client_with_two_endpoints(runner)
    try:
        rt.start_service(client_host, runner, "client")
        before_pid = rt.wait_for_stable_xray_pid(client_host)
        _rewrite_live_api_listen(client_host, "127.0.0.1:1")
        desired_before = helpers.read_text(client_host, helpers.CLIENT_CONFIG_FILE)
        live_before = helpers.read_text(client_host, helpers.CLIENT_LIVE_DIR / "xray.json")

        result = runner("client", "update", primary_tag, "--user", NEW_USER, "--password", NEW_PASSWORD)

        assert result.rc != 0, f"client update unexpectedly succeeded.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        output = f"{result.stdout}\n{result.stderr}".lower()
        assert "runtime apply" in output or "connection refused" in output or "connect" in output
        assert helpers.read_text(client_host, helpers.CLIENT_CONFIG_FILE) == desired_before
        assert helpers.read_text(client_host, helpers.CLIENT_LIVE_DIR / "xray.json") == live_before
        rt.assert_same_xray_pid(client_host, before_pid, "client-update-api-unavailable-pid-changed")
        rt.assert_apply_error(client_host, "client")
    finally:
        rt.stop_service(runner, "client")


def _rewrite_live_api_listen(host, listen: str) -> None:
    path = helpers.CLIENT_LIVE_DIR / "xray.json"
    data = helpers.read_json(host, path)
    data.setdefault("api", {})["listen"] = listen
    helpers.write_text(host, path, json.dumps(data, indent=2) + "\n")
