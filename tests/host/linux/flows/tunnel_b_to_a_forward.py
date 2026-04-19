from __future__ import annotations

from tests.host.linux.flows import tunnel_b_to_a_fixture as fixture
from tests.host.tunnel import common as tunnel_common


def assert_forward_tunnel_operational(env: dict) -> None:
    client_runner = env["client_runner"]

    with fixture.active_tunnel_sessions(env):
        ping_result = client_runner(
            "ping",
            fixture.SERVER_IP,
            "--tunnel",
            "--count",
            "3",
            check=True,
        )
        tunnel_common.assert_zero_loss(ping_result, "through SOCKS tunnel")
        fixture.verify_heartbeat_state(env)
        fixture.run_server_state_watch(env)
    fixture.wait_for_dead_entry(env)
    fixture.exercise_client_forward_diagnostics(env)
    fixture.exercise_server_forward_diagnostics(env)

