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


def test_runtime_client_endpoint_disable_enable_preserves_desired_links(client_host):
    runner = rt.xp2p_runner(client_host)
    primary_tag, secondary_tag, reverse_tag = setup.install_client_with_two_endpoints(runner)
    try:
        rt.start_service(client_host, runner, "client")
        before_pid = rt.wait_for_stable_xray_pid(client_host)
        live = rt.wait_for_live_xray(client_host, "client")
        assert {primary_tag, secondary_tag} <= rt.outbound_tags(live)
        helpers.assert_redirect_rule(live, setup.CLIENT_REDIRECT_CIDR, primary_tag)

        runner("client", "disable", primary_tag, check=True)
        rt.wait_for_apply_clear(client_host)
        rt.assert_same_xray_pid(client_host, before_pid, "client-endpoint-disable-pid-changed")
        desired = setup.client_desired(client_host)
        assert setup.endpoint_by_tag(desired, primary_tag).get("disabled") is True
        assert setup.endpoint_by_tag(desired, secondary_tag).get("disabled") is not True
        assert setup.redirects_for_tag(desired, primary_tag), "Desired redirects for disabled endpoint were removed"
        assert reverse_tag in (desired.get("reverse") or {}), "Desired reverse record for disabled endpoint was removed"
        live = rt.wait_for_live_xray(client_host, "client")
        assert primary_tag not in rt.outbound_tags(live)
        assert secondary_tag in rt.outbound_tags(live)
        rt.assert_no_route_to(live, primary_tag)
        rt.assert_apply_clean(client_host)

        rt.restart_service(client_host, runner, "client")
        assert primary_tag not in rt.outbound_tags(rt.wait_for_live_xray(client_host, "client"))

        before_pid = rt.wait_for_stable_xray_pid(client_host)
        runner("client", "enable", primary_tag, check=True)
        rt.wait_for_apply_clear(client_host)
        rt.assert_same_xray_pid(client_host, before_pid, "client-endpoint-enable-pid-changed")
        desired = setup.client_desired(client_host)
        assert not setup.endpoint_by_tag(desired, primary_tag).get("disabled", False)
        live = rt.wait_for_live_xray(client_host, "client")
        assert {primary_tag, secondary_tag} <= rt.outbound_tags(live)
        helpers.assert_redirect_rule(live, setup.CLIENT_REDIRECT_CIDR, primary_tag)
        rt.restart_service(client_host, runner, "client")
        assert primary_tag in rt.outbound_tags(rt.wait_for_live_xray(client_host, "client"))
    finally:
        rt.stop_service(runner, "client")
