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


def test_runtime_server_user_disable_enable_preserves_pid_and_persistence(server_host):
    runner = rt.xp2p_runner(server_host)
    user_alpha, user_bravo = setup.install_server_with_users(runner)
    try:
        rt.start_service(server_host, runner, "server")
        before_pid = rt.wait_for_stable_xray_pid(server_host)
        live = rt.wait_for_live_xray(server_host, "server")
        assert {user_alpha, user_bravo} <= rt.trojan_user_ids(live)

        runner("server", "user", "disable", user_alpha, check=True)
        rt.wait_for_apply_clear(server_host)
        rt.assert_same_xray_pid(server_host, before_pid, "server-user-disable-pid-changed")
        desired = setup.server_desired(server_host)
        users = setup.server_users_by_label(desired)
        assert users[user_alpha].get("disabled") is True
        assert not users[user_bravo].get("disabled", False)
        live = rt.wait_for_live_xray(server_host, "server")
        assert user_alpha not in rt.trojan_user_ids(live)
        assert user_bravo in rt.trojan_user_ids(live)
        reverse = desired.get("reverse_channels") or {}
        assert helpers.expected_reverse_tag(user_alpha, setup.SERVER_HOST) in reverse
        rt.assert_apply_clean(server_host)

        rt.restart_service(server_host, runner, "server")
        assert user_alpha not in rt.trojan_user_ids(rt.wait_for_live_xray(server_host, "server"))

        before_pid = rt.wait_for_stable_xray_pid(server_host)
        runner("server", "user", "enable", user_alpha, check=True)
        rt.wait_for_apply_clear(server_host)
        rt.assert_same_xray_pid(server_host, before_pid, "server-user-enable-pid-changed")
        desired = setup.server_desired(server_host)
        users = setup.server_users_by_label(desired)
        assert not users[user_alpha].get("disabled", False)
        live = rt.wait_for_live_xray(server_host, "server")
        assert {user_alpha, user_bravo} <= rt.trojan_user_ids(live)
        rt.restart_service(server_host, runner, "server")
        assert {user_alpha, user_bravo} <= rt.trojan_user_ids(rt.wait_for_live_xray(server_host, "server"))
    finally:
        rt.stop_service(runner, "server")
