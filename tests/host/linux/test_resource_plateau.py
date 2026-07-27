from __future__ import annotations

import os
import time
from contextlib import ExitStack

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import _netem_lab as netem
from tests.host.linux import _resource_plateau as plateau
from tests.host.linux import _resource_plateau_gate as gate
from tests.host.linux import _resource_plateau_nightly as nightly_topology
from tests.host.linux import _resource_plateau_scenarios as scenarios
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
        gate.NIGHTLY_SAMPLE_INTERVAL_SECONDS if nightly else gate.QUICK_SAMPLE_INTERVAL_SECONDS,
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
        scenarios.set_control_status(env, None)
        with ExitStack() as stack:
            stack.enter_context(
                fixture.ip_alias(env["server_host"], f"{gate.SECOND_ENDPOINT_IP}/32")
            )
            scenarios.add_second_endpoint(env)
            sessions = stack.enter_context(
                fixture.active_tunnel_sessions(
                    env,
                    runtime_metrics=True,
                    test_heartbeat_interval="" if nightly else "250ms",
                    server_process_env={
                        "XP2P_TEST_MODE": "1",
                        "XP2P_TEST_CONTROL_STATUS_FILE": scenarios.CONTROL_STATUS_FILE,
                    },
                    client_process_env={
                        "XP2P_TEST_MODE": "1",
                        "XP2P_TEST_SUBSCRIPTION_INTERVAL": "250ms",
                    } if not nightly else None,
                )
            )
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
            aux_session["xray_pid"] = plateau.xray_pid(aux_host)
            extra_clients = (
                stack.enter_context(nightly_topology.extra_client_sessions(env, aux_host))
                if nightly
                else []
            )
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
                "aux_client_xray": (aux_host, aux_session["xray_pid"], fixture.SERVER_IP, ""),
                "server_xp2p": (
                    env["server_host"], sessions["server"]["pid"], fixture.CLIENT_IP,
                    sessions["server"]["runtime_metrics"],
                ),
                "server_xray": (env["server_host"], plateau.xray_pid(env["server_host"]), fixture.CLIENT_IP, ""),
            }
            for index, client in enumerate(extra_clients):
                owners[f"nightly_client_{index}_xp2p"] = (
                    client["host"],
                    client["pid"],
                    client["peer"],
                    client["runtime_metrics"],
                )
                owners[f"nightly_client_{index}_xray"] = (
                    client["host"],
                    client["xray_pid"],
                    client["peer"],
                    "",
                )
            payload["topology"] = {
                "client_count": 2 + len(extra_clients),
                "mixed_legacy_clients": "unsupported: no legacy binary fixture is available",
            }
            assert payload["topology"]["client_count"] == (5 if nightly else 2)
            payload["samples"] = {name: [] for name in owners}
            phase_count = sample_count // len(gate.PHASE_NAMES)
            if phase_count < 3:
                pytest.fail(f"sample count must provide at least three samples for each of {gate.PHASE_NAMES}")
            payload["phases"] = {}
            gate.collect_phase(payload, "stable_rotation_absent", owners, phase_count, sample_interval)
            env["server_runner"]("server", "user", "rotate", env["client_user"], check=True)
            gate.collect_phase(payload, "rotation_pending", owners, phase_count, sample_interval)
            scenarios.set_control_status(env, 503)
            non_200_count = scenarios.control_status_count(env)
            gate.collect_phase(
                payload, "non_200", owners, phase_count, sample_interval,
            )
            scenarios.assert_xp2p_non_200(env, non_200_count)
            scenarios.set_control_status(env, None)
            with netem.netem_degradation(
                env["client_host"],
                fixture.SERVER_IP,
                "delay 2500ms 500ms loss 30% limit 1000",
            ):
                gate.collect_phase(
                    payload,
                    "timeout_packet_loss",
                    owners,
                    phase_count,
                    sample_interval,
                )
            with netem.netem_degradation(
                env["client_host"],
                fixture.SERVER_IP,
                "loss 100% limit 1000",
            ):
                gate.collect_phase(
                    payload,
                    "full_network_loss",
                    owners,
                    phase_count,
                    sample_interval,
                )
                recovery_baselines = {
                    "client": scenarios.heartbeat_attempts(env["client_host"]),
                    "aux": scenarios.heartbeat_attempts(aux_host),
                }
            netem.wait_for_no_netem(env["client_host"], fixture.SERVER_IP)
            scenarios.wait_for_recovery(env, aux_host, recovery_baselines)
            gate.collect_phase(
                payload,
                "recovered",
                owners,
                sample_count - (phase_count * (len(gate.PHASE_NAMES) - 1)),
                sample_interval,
            )
            gate.assess_phases(payload)
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
