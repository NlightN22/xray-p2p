from __future__ import annotations

import os

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import _runtime_disable as rt
from tests.host.linux import _runtime_disable_setup as setup

SKIP_RUNTIME_DISABLE = os.environ.get("XP2P_RUN_SERVICE_CLI_TESTS", "").strip().lower() not in {
    "1",
    "true",
    "yes",
}
pytestmark = [
    pytest.mark.host,
    pytest.mark.linux,
    pytest.mark.skipif(SKIP_RUNTIME_DISABLE, reason="runtime disable service tests are opt-in"),
]


def test_runtime_client_reverse_soft_disable_keeps_bridge(client_host):
    runner = rt.xp2p_runner(client_host)
    primary_tag, _, reverse_tag = setup.install_client_with_two_endpoints(runner)
    try:
        rt.start_service(client_host, runner, "client")
        before_pid = rt.wait_for_stable_xray_pid(client_host)

        runner("client", "reverse", "disable", reverse_tag, check=True)
        rt.wait_for_apply_clear(client_host)
        rt.assert_same_xray_pid(client_host, before_pid, "client-reverse-disable-pid-changed")
        desired = setup.client_desired(client_host)
        assert (desired.get("reverse") or {})[reverse_tag].get("disabled") is True
        live = rt.wait_for_live_xray(client_host, "client")
        rt.assert_client_reverse_bridge_without_rules(live, reverse_tag)
        rt.restart_service(client_host, runner, "client")
        rt.assert_client_reverse_bridge_without_rules(
            rt.wait_for_live_xray(client_host, "client"),
            reverse_tag,
        )

        before_pid = rt.wait_for_stable_xray_pid(client_host)
        runner("client", "reverse", "enable", reverse_tag, check=True)
        rt.wait_for_apply_clear(client_host)
        rt.assert_same_xray_pid(client_host, before_pid, "client-reverse-enable-pid-changed")
        desired = setup.client_desired(client_host)
        assert not (desired.get("reverse") or {})[reverse_tag].get("disabled", False)
        live = rt.wait_for_live_xray(client_host, "client")
        helpers.assert_client_reverse_artifacts(live, reverse_tag, primary_tag)
        rt.assert_apply_clean(client_host)
    finally:
        rt.stop_service(runner, "client")
