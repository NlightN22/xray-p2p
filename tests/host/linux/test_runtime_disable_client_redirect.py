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


def test_runtime_client_redirect_disable_enable_updates_live_routes(client_host):
    runner = rt.xp2p_runner(client_host)
    primary_tag, _, _ = setup.install_client_with_two_endpoints(runner)
    try:
        rt.start_service(client_host, runner, "client")
        before_pid = rt.wait_for_stable_xray_pid(client_host)

        runner(
            "client",
            "redirect",
            "disable",
            "--cidr",
            setup.CLIENT_REDIRECT_CIDR,
            "--tag",
            primary_tag,
            "--quiet",
            check=True,
        )
        rt.wait_for_apply_clear(client_host)
        assert rt.xray_pid(client_host) == before_pid
        desired = setup.client_desired(client_host)
        cidr_redirects = [entry for entry in setup.redirects_for_tag(desired, primary_tag) if entry.get("cidr")]
        assert cidr_redirects and cidr_redirects[0].get("disabled") is True
        live = rt.wait_for_live_xray(client_host, "client")
        helpers.assert_no_redirect_rule(live, setup.CLIENT_REDIRECT_CIDR, primary_tag)
        helpers.assert_domain_redirect_rule(live, setup.CLIENT_REDIRECT_DOMAIN, primary_tag)
        rt.restart_service(client_host, runner, "client")
        helpers.assert_no_redirect_rule(
            rt.wait_for_live_xray(client_host, "client"),
            setup.CLIENT_REDIRECT_CIDR,
            primary_tag,
        )

        before_pid = rt.wait_for_stable_xray_pid(client_host)
        runner(
            "client",
            "redirect",
            "enable",
            "--cidr",
            setup.CLIENT_REDIRECT_CIDR,
            "--tag",
            primary_tag,
            "--quiet",
            check=True,
        )
        rt.wait_for_apply_clear(client_host)
        assert rt.xray_pid(client_host) == before_pid
        live = rt.wait_for_live_xray(client_host, "client")
        helpers.assert_redirect_rule(live, setup.CLIENT_REDIRECT_CIDR, primary_tag)
        rt.assert_apply_clean(client_host)
    finally:
        rt.stop_service(runner, "client")
