from __future__ import annotations

import time

import pytest

from tests.host.linux import _resource_plateau as plateau
from tests.host.linux import _resource_plateau_gate as gate
from tests.host.linux import _resource_plateau_scenarios as scenarios
from tests.host.linux.flows import tunnel_b_to_a_fixture as fixture

pytestmark = [pytest.mark.host, pytest.mark.linux]
tunnel_environment = fixture.tunnel_environment


@pytest.mark.parametrize(
    ("values", "limit", "passes"),
    [
        ([10, 11, 10, 12, 11], plateau.PlateauLimit(3, 0.5), True),
        ([10, 20, 30, 40, 50], plateau.PlateauLimit(50, 1), False),
    ],
)
def test_plateau_assessment_detects_linear_growth(values, limit, passes):
    if passes:
        plateau.assess(values, limit)
    else:
        with pytest.raises(AssertionError, match="did not plateau"):
            plateau.assess(values, limit)


def test_process_sampling_fails_closed_when_pid_disappears(client_host):
    with pytest.raises(AssertionError, match="process 999999 disappeared"):
        plateau.process_sample(client_host, 999999, fixture.SERVER_IP)


@pytest.mark.parametrize(
    ("owner", "missing"),
    [
        ("client_xp2p", "control_http_clients"),
        ("server_xp2p", "control_connections_current"),
    ],
)
def test_phase_gate_rejects_missing_owner_metric(owner, missing):
    metrics = {
        name: 1
        for name in gate.LIMITS
    } | {
        name: 1
        for name in gate.GO_LIMITS
    }
    metrics |= {
        name: 1
        for name in (
            gate.SERVER_GO_LIMITS if owner == "server_xp2p" else gate.CLIENT_GO_LIMITS
        )
    }
    del metrics[missing]
    payload = {
        "samples": {owner: [metrics.copy() for _ in range(3)]},
        "phases": {"stable": {"start": 0, "end": 3}},
    }
    with pytest.raises(AssertionError, match=f"{owner}/{missing}: metric missing"):
        gate.assess_phases(payload)


def test_integration_gate_rejects_xp2p_control_transport_leak(tunnel_environment):
    env = tunnel_environment
    scenarios.set_control_status(env, 503)
    try:
        with fixture.active_tunnel_sessions(
            env,
            runtime_metrics=True,
            server_process_env={
                "XP2P_TEST_MODE": "1",
                "XP2P_TEST_CONTROL_STATUS_FILE": scenarios.CONTROL_STATUS_FILE,
            },
            client_process_env={
                "XP2P_TEST_MODE": "1",
                "XP2P_TEST_SUBSCRIPTION_INTERVAL": "100ms",
                "XP2P_TEST_CONTROL_TRANSPORT_LEAK": "1",
            },
        ) as sessions:
            samples = []
            for _ in range(8):
                samples.append(plateau.process_sample(
                    env["client_host"], sessions["client"]["pid"], fixture.SERVER_IP,
                    sessions["client"]["runtime_metrics"],
                ))
                time.sleep(1)
            with pytest.raises(AssertionError, match="did not plateau"):
                plateau.assess(
                    [sample["control_http_clients"] for sample in samples],
                    plateau.PlateauLimit(maximum_range=4, maximum_slope=0.5),
                )
    finally:
        scenarios.set_control_status(env, None)


def test_client_restart_and_server_shutdown_release_previous_owners(tunnel_environment):
    env = tunnel_environment
    with fixture.active_tunnel_sessions(env) as first:
        first_pids = _owner_pids(env, first)
    gate.assert_pids_gone(env, first_pids)
    gate.assert_owner_shutdown(env, first, env["client_host"])

    with fixture.active_tunnel_sessions(env) as second:
        fixture.verify_heartbeat_state(env)
        assert second["client"]["pid"] != first_pids["client_xp2p"]
        assert second["server"]["pid"] != first_pids["server_xp2p"]
        second_pids = _owner_pids(env, second)
    gate.assert_pids_gone(env, second_pids)
    gate.assert_owner_shutdown(env, second, env["client_host"])


def _owner_pids(env: dict, sessions: dict) -> dict[str, int]:
    return {
        "client_xp2p": sessions["client"]["pid"],
        "client_xray": plateau.xray_pid(env["client_host"]),
        "server_xp2p": sessions["server"]["pid"],
        "server_xray": plateau.xray_pid(env["server_host"]),
    }
