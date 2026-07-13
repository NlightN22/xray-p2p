from __future__ import annotations

import os

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import _netem_lab as netem
from tests.host.linux.flows import tunnel_b_to_a_fixture as fixture
from tests.host.host_common.polling import wait_until
from tests.host.tunnel import common as tunnel_common

SERVER_TUNNEL_PORT = 58443
FULL_LOSS_NETEM = "loss 100% limit 1000"

pytestmark = [
    pytest.mark.host,
    pytest.mark.linux,
    pytest.mark.skipif(
        os.environ.get("XP2P_RUN_HEARTBEAT_STORM_TESTS") != "1",
        reason="set XP2P_RUN_HEARTBEAT_STORM_TESTS=1 to run netem heartbeat storm lab",
    ),
]
tunnel_environment = fixture.tunnel_environment


def test_heartbeat_netem_lab_collects_socket_metrics(tunnel_environment):
    env = tunnel_environment
    env["server_ip"] = fixture.SERVER_IP

    with fixture.active_tunnel_sessions(env):
        baseline = env["client_runner"](
            "ping",
            fixture.SERVER_IP,
            "--tunnel",
            "--count",
            "3",
            check=True,
        )
        tunnel_common.assert_zero_loss(baseline, "baseline heartbeat tunnel")
        reverse_baseline = _reverse_socks_ping(env, check=True)
        tunnel_common.assert_zero_loss(reverse_baseline, "baseline reverse SOCKS tunnel")
        fixture.verify_heartbeat_state(env)

        before = _sample(env)
        with netem.netem_degradation(env["client_host"], fixture.SERVER_IP):
            probes = netem.run_heartbeat_probe_burst(env, attempts=10, timeout_seconds=1)
            reverse_during = _reverse_socks_ping(env, check=False)
            during = _sample(env)

        netem.wait_for_no_netem(env["client_host"], fixture.SERVER_IP)
        recovery = env["client_runner"](
            "ping",
            fixture.SERVER_IP,
            "--tunnel",
            "--count",
            "3",
            "--timeout",
            "5",
            check=True,
        )
        tunnel_common.assert_zero_loss(recovery, "recovered heartbeat tunnel")
        reverse_recovery = _reverse_socks_ping(env, check=True)
        tunnel_common.assert_zero_loss(reverse_recovery, "recovered reverse SOCKS tunnel")
        after = _sample(env)

    failed = sum(1 for item in probes if int(item["rc"]) != 0)
    assert failed > 0, "netem lab did not force any heartbeat probe failures"
    assert netem.received_ping_replies(reverse_during.stdout) > 0, (
        "reverse SOCKS probe received no replies while heartbeat was degraded.\n"
        f"STDOUT:\n{reverse_during.stdout}\nSTDERR:\n{reverse_during.stderr}"
    )
    assert during["sockets"]["total"] >= before["sockets"]["total"]
    assert after["fd_count"] > 0
    print(f"heartbeat netem metrics: before={before} during={during} after={after} failed_probes={failed}")


def test_heartbeat_netem_lab_status_transitions(tunnel_environment):
    env = tunnel_environment
    env["server_ip"] = fixture.SERVER_IP

    with fixture.active_tunnel_sessions(env):
        baseline = env["client_runner"](
            "ping",
            fixture.SERVER_IP,
            "--tunnel",
            "--count",
            "3",
            check=True,
        )
        tunnel_common.assert_zero_loss(baseline, "baseline heartbeat tunnel")
        alive_before = _wait_for_client_status(env, "alive")

        with netem.netem_degradation(env["client_host"], fixture.SERVER_IP, FULL_LOSS_NETEM):
            failed = netem.run_heartbeat_probe_burst(env, attempts=4, timeout_seconds=1)
            reverse_during = _reverse_socks_ping(env, check=False)
            dead_during = _wait_for_client_status(env, "dead", ttl="2s", timeout_seconds=20.0)

        netem.wait_for_no_netem(env["client_host"], fixture.SERVER_IP)
        recovery = env["client_runner"](
            "ping",
            fixture.SERVER_IP,
            "--tunnel",
            "--count",
            "3",
            "--timeout",
            "5",
            check=True,
        )
        tunnel_common.assert_zero_loss(recovery, "recovered heartbeat tunnel")
        alive_after = _wait_for_client_status(env, "alive")

    assert all(int(item["rc"]) != 0 for item in failed), "full-loss netem did not fail all heartbeat probes"
    assert netem.received_ping_replies(reverse_during.stdout) == 0, (
        "reverse SOCKS unexpectedly received replies during full-loss netem.\n"
        f"STDOUT:\n{reverse_during.stdout}\nSTDERR:\n{reverse_during.stderr}"
    )
    print(f"heartbeat status transitions: before={alive_before} during={dead_during} after={alive_after}")


def _sample(env: dict) -> dict:
    client_host = env["client_host"]
    return {
        "sockets": netem.socket_snapshot(client_host, fixture.SERVER_IP, SERVER_TUNNEL_PORT),
        "fd_count": netem.xray_fd_count(client_host),
        "client_state": _client_state(env),
    }


def _client_state(env: dict) -> str:
    result = env["client_runner"](
        "client",
        "state",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--ttl",
        "3s",
        check=True,
    )
    return result.stdout or ""


def _client_state_row(env: dict, *, ttl: str = "3s") -> dict[str, str]:
    result = env["client_runner"](
        "client",
        "state",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--ttl",
        ttl,
        check=True,
    )
    rows = tunnel_common.parse_state_rows(result.stdout or "")
    for row in rows:
        if row.get("HOST", "").strip() == fixture.SERVER_IP:
            return row
    pytest.fail(f"client state row for {fixture.SERVER_IP} not found.\nSTDOUT:\n{result.stdout}")


def _wait_for_client_status(
    env: dict,
    status: str,
    *,
    ttl: str = "3s",
    timeout_seconds: float = 30.0,
) -> dict[str, str]:
    expected = status.strip().lower()

    def _poll():
        row = _client_state_row(env, ttl=ttl)
        if row.get("STATUS", "").strip().lower() == expected:
            return row
        return None

    try:
        return wait_until(
            f"client heartbeat status to become {expected}",
            _poll,
            timeout_seconds=timeout_seconds,
            poll_interval=1.0,
        ).value
    except TimeoutError as exc:
        row = _client_state_row(env, ttl=ttl)
        pytest.fail(f"{exc}\nLast client state row: {row}")


def _reverse_socks_ping(env: dict, *, check: bool):
    server_socks_addr = f"127.0.0.1:{fixture.socks_port(env['server_host'], helpers.SERVER_LIVE_DIR / 'xray.json')}"
    return env["server_runner"](
        "ping",
        fixture.CLIENT_IP,
        f"--tunnel={server_socks_addr}",
        "--port",
        str(fixture.CLIENT_DIAGNOSTICS_PORT),
        "--count",
        "5",
        "--timeout",
        "5",
        check=check,
    )
