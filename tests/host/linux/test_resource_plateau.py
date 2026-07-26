from __future__ import annotations

import os
import time

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import _resource_plateau as plateau
from tests.host.linux.flows import tunnel_b_to_a_fixture as fixture

pytestmark = [pytest.mark.host, pytest.mark.linux]
tunnel_environment = fixture.tunnel_environment

QUICK_SAMPLES = 24
NIGHTLY_SAMPLES = 720
SAMPLE_INTERVAL_SECONDS = 5.0
WARMUP_SECONDS = 30.0
LIMITS = {
    "rss_kib": plateau.PlateauLimit(32 * 1024, 256),
    "threads": plateau.PlateauLimit(8, 0.1),
    "fd": plateau.PlateauLimit(12, 0.1),
    "socket_fd": plateau.PlateauLimit(8, 0.1),
    "established": plateau.PlateauLimit(8, 0.1),
}


@pytest.mark.skipif(
    os.environ.get("XP2P_RUN_RESOURCE_PLATEAU") != "1",
    reason="set XP2P_RUN_RESOURCE_PLATEAU=1 to run the resource plateau gate",
)
def test_control_plane_resources_reach_plateau(tunnel_environment):
    env = tunnel_environment
    nightly = os.environ.get("XP2P_RESOURCE_PLATEAU_PROFILE") == "nightly"
    sample_count = _positive_int("XP2P_RESOURCE_PLATEAU_SAMPLES", NIGHTLY_SAMPLES if nightly else QUICK_SAMPLES)
    sample_interval = _positive_float("XP2P_RESOURCE_PLATEAU_SAMPLE_INTERVAL", SAMPLE_INTERVAL_SECONDS)
    warmup = _positive_float("XP2P_RESOURCE_PLATEAU_WARMUP", WARMUP_SECONDS)

    payload = {"profile": "nightly" if nightly else "quick", "samples": {}, "assessments": {}}
    try:
        with fixture.active_tunnel_sessions(env) as sessions:
            time.sleep(warmup)
            owners = {
                "client_xp2p": (env["client_host"], sessions["client"]["pid"], fixture.SERVER_IP),
                "client_xray": (env["client_host"], plateau.xray_pid(env["client_host"]), fixture.SERVER_IP),
                "server_xp2p": (env["server_host"], sessions["server"]["pid"], fixture.CLIENT_IP),
                "server_xray": (env["server_host"], plateau.xray_pid(env["server_host"]), fixture.CLIENT_IP),
            }
            for name, (host, pid, peer_ip) in owners.items():
                payload["samples"][name] = plateau.collect(host, pid, peer_ip, sample_count, sample_interval)

            for owner, samples in payload["samples"].items():
                payload["assessments"][owner] = {}
                for metric, limit in LIMITS.items():
                    values = [sample[metric] for sample in samples]
                    payload["assessments"][owner][metric] = plateau.assess(values, limit)
    except BaseException:
        for role, host in (("client", env["client_host"]), ("server", env["server_host"])):
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


def _positive_int(name: str, default: int) -> int:
    value = int(os.environ.get(name, str(default)))
    if value < 3:
        pytest.fail(f"{name} must be at least 3")
    return value


def _positive_float(name: str, default: float) -> float:
    value = float(os.environ.get(name, str(default)))
    if value <= 0:
        pytest.fail(f"{name} must be positive")
    return value
