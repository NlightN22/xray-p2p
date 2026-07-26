from __future__ import annotations

import os
import time
from contextlib import ExitStack

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import _netem_lab as netem
from tests.host.linux import _resource_plateau as plateau
from tests.host.linux import _resource_plateau_gate as gate
from tests.host.linux import env as linux_env
from tests.host.linux.flows import tunnel_b_to_a_fixture as fixture

pytestmark = [pytest.mark.host, pytest.mark.linux]
tunnel_environment = fixture.tunnel_environment

@pytest.mark.skipif(
    os.environ.get("XP2P_RUN_RESOURCE_PLATEAU") != "1",
    reason="set XP2P_RUN_RESOURCE_PLATEAU=1 to run the resource plateau gate",
)
def test_control_plane_resources_reach_plateau(tunnel_environment, aux_host):
    env = tunnel_environment
    nightly = os.environ.get("XP2P_RESOURCE_PLATEAU_PROFILE") == "nightly"
    sample_count = gate.positive_int(
        "XP2P_RESOURCE_PLATEAU_SAMPLES",
        gate.NIGHTLY_SAMPLES if nightly else gate.QUICK_SAMPLES,
    )
    sample_interval = gate.positive_float(
        "XP2P_RESOURCE_PLATEAU_SAMPLE_INTERVAL",
        gate.SAMPLE_INTERVAL_SECONDS,
    )
    warmup = gate.positive_float("XP2P_RESOURCE_PLATEAU_WARMUP", gate.WARMUP_SECONDS)

    payload = {"profile": "nightly" if nightly else "quick", "samples": {}, "assessments": {}}
    sessions = None
    try:
        aux_runner = fixture.runner(aux_host)
        aux_credential = env["server_runner"](
            "server", "user", "add", "--json",
            "--path", env["server_install_path"],
            "--config-dir", helpers.SERVER_CONFIG_DIR_NAME,
            "--id", "resource-plateau-aux@example.com",
            "--host", fixture.SERVER_IP,
            check=True,
        )
        aux_link = helpers.parse_json_credential(aux_credential.stdout or "")["link"]
        aux_runner(
            "client", "install",
            "--path", helpers.INSTALL_ROOT.as_posix(),
            "--config-dir", helpers.CLIENT_CONFIG_DIR_NAME,
            "--mode", "proxy",
            "--link", aux_link,
            "--force",
            check=True,
        )
        with ExitStack() as stack:
            sessions = stack.enter_context(fixture.active_tunnel_sessions(env, runtime_metrics=True))
            aux_session = stack.enter_context(
                linux_env.xp2p_run_session(
                    aux_host,
                    "client",
                    helpers.INSTALL_ROOT.as_posix(),
                    helpers.CLIENT_CONFIG_DIR_NAME,
                    runtime_metrics_file="/tmp/xp2p-client-runtime.metrics",
                )
            )
            aux_session["runtime_metrics"] = "/tmp/xp2p-client-runtime.metrics"
            time.sleep(warmup)
            owners = {
                "client_xp2p": (
                    env["client_host"], sessions["client"]["pid"], fixture.SERVER_IP,
                    sessions["client"]["runtime_metrics"],
                ),
                "client_xray": (env["client_host"], plateau.xray_pid(env["client_host"]), fixture.SERVER_IP, ""),
                "aux_client_xp2p": (
                    aux_host, aux_session["pid"], fixture.SERVER_IP, aux_session["runtime_metrics"],
                ),
                "aux_client_xray": (aux_host, plateau.xray_pid(aux_host), fixture.SERVER_IP, ""),
                "server_xp2p": (
                    env["server_host"], sessions["server"]["pid"], fixture.CLIENT_IP,
                    sessions["server"]["runtime_metrics"],
                ),
                "server_xray": (env["server_host"], plateau.xray_pid(env["server_host"]), fixture.CLIENT_IP, ""),
            }
            payload["samples"] = {name: [] for name in owners}
            phase_count = max(3, sample_count // 4)
            payload["phases"] = {}
            gate.collect_phase(payload, "stable_rotation_absent", owners, phase_count, sample_interval)
            env["server_runner"]("server", "user", "rotate", env["client_user"], check=True)
            gate.collect_phase(payload, "rotation_pending", owners, phase_count, sample_interval)
            with netem.netem_degradation(
                env["client_host"],
                fixture.SERVER_IP,
                "delay 500ms 200ms 30% loss 25% limit 1000",
            ):
                gate.collect_phase(payload, "degraded", owners, phase_count, sample_interval)
            netem.wait_for_no_netem(env["client_host"], fixture.SERVER_IP)
            gate.collect_phase(
                payload,
                "recovered",
                owners,
                sample_count - (phase_count * 3),
                sample_interval,
            )

            for owner, samples in payload["samples"].items():
                payload["assessments"][owner] = {}
                limits = gate.LIMITS | (gate.GO_LIMITS if owner.endswith("_xp2p") else {})
                for metric, limit in limits.items():
                    values = [sample[metric] for sample in samples]
                    payload["assessments"][owner][metric] = plateau.assess(values, limit)
        gate.assert_owner_shutdown(env, sessions, aux_host)
    except BaseException:
        for role, host in (
            ("client", env["client_host"]),
            ("aux-client", aux_host),
            ("server", env["server_host"]),
        ):
            helpers.dump_failure_state(host, f"resource-plateau-{role}")
        path = plateau.write_artifact("resource-plateau-failure", payload)
        print(f"resource plateau failure artifact: {path}")
        raise

    path = plateau.write_artifact("resource-plateau", payload)
    print(f"resource plateau result: {payload['assessments']}")
    print(f"resource plateau artifact: {path}")


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


def test_client_restart_and_server_shutdown_release_previous_owners(tunnel_environment):
    env = tunnel_environment
    with fixture.active_tunnel_sessions(env) as first:
        first_pids = {
            "client_xp2p": first["client"]["pid"],
            "client_xray": plateau.xray_pid(env["client_host"]),
            "server_xp2p": first["server"]["pid"],
            "server_xray": plateau.xray_pid(env["server_host"]),
        }
    gate.assert_pids_gone(env, first_pids)
    gate.assert_owner_shutdown(env, first, env["client_host"])

    with fixture.active_tunnel_sessions(env) as second:
        fixture.verify_heartbeat_state(env)
        assert second["client"]["pid"] != first_pids["client_xp2p"]
        assert second["server"]["pid"] != first_pids["server_xp2p"]
