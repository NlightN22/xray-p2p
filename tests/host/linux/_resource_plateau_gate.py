from __future__ import annotations

import os
import time

import pytest

from tests.host.linux import _resource_plateau as plateau
from tests.host.linux.flows import tunnel_b_to_a_fixture as fixture

QUICK_SAMPLES = 120
NIGHTLY_SAMPLES = 180
SOAK_SAMPLES = 1440
QUICK_SAMPLE_INTERVAL_SECONDS = 1.0
NIGHTLY_SAMPLE_INTERVAL_SECONDS = 1.0
SOAK_SAMPLE_INTERVAL_SECONDS = 5.0
WARMUP_SECONDS = 30.0
EXPANDED_WARMUP_SECONDS = 75.0
AUX_CLIENT_IP = "10.62.10.13"
SECOND_ENDPOINT_IP = "10.62.10.14"
PHASE_NAMES = (
    "stable_rotation_absent",
    "rotation_pending",
    "non_200",
    "timeout_packet_loss",
    "full_network_loss",
    "recovered",
)
PROFILES = {
    "quick": (QUICK_SAMPLES, QUICK_SAMPLE_INTERVAL_SECONDS, WARMUP_SECONDS, False, True),
    "nightly": (
        NIGHTLY_SAMPLES,
        NIGHTLY_SAMPLE_INTERVAL_SECONDS,
        EXPANDED_WARMUP_SECONDS,
        True,
        True,
    ),
    "soak": (
        SOAK_SAMPLES,
        SOAK_SAMPLE_INTERVAL_SECONDS,
        EXPANDED_WARMUP_SECONDS,
        True,
        False,
    ),
}
LIMITS = {
    "rss_kib": plateau.PlateauLimit(32 * 1024, 256),
    "threads": plateau.PlateauLimit(8, 0.25),
    "fd": plateau.PlateauLimit(12, 0.2),
    "socket_fd": plateau.PlateauLimit(8, 0.2),
    "pipe_fd": plateau.PlateauLimit(8, 0.2),
    "anon_fd": plateau.PlateauLimit(8, 0.2),
    "tcp_total": plateau.PlateauLimit(8, 0.2),
    "tcp_estab": plateau.PlateauLimit(8, 0.2),
    "tcp_peer": plateau.PlateauLimit(8, 0.2),
    "cgroup_memory": plateau.PlateauLimit(64 * 1024 * 1024, 512 * 1024),
}
GO_LIMITS = {
    "go_heap_alloc": plateau.PlateauLimit(32 * 1024 * 1024, 256 * 1024),
    "go_heap_sys": plateau.PlateauLimit(32 * 1024 * 1024, 256 * 1024),
    "go_goroutines": plateau.PlateauLimit(16, 0.1),
}
CLIENT_GO_LIMITS = {
    "control_http_clients": plateau.PlateauLimit(4, 0.1),
}
SERVER_GO_LIMITS = {
    "control_connections_active": plateau.PlateauLimit(8, 0.1),
    "control_connections_idle": plateau.PlateauLimit(8, 0.1),
    "control_connections_current": plateau.PlateauLimit(8, 0.1),
    "control_connections_peak": plateau.PlateauLimit(8, 0.1),
}


def positive_int(name: str, default: int) -> int:
    value = int(os.environ.get(name, str(default)))
    if value < 3:
        pytest.fail(f"{name} must be at least 3")
    return value


def positive_float(name: str, default: float) -> float:
    value = float(os.environ.get(name, str(default)))
    if value <= 0:
        pytest.fail(f"{name} must be positive")
    return value


def resolve_profile(profile: str) -> tuple[int, float, float, bool, bool]:
    if profile not in PROFILES:
        pytest.fail(f"unsupported XP2P_RESOURCE_PLATEAU_PROFILE: {profile}")
    samples, interval, warmup, expanded, accelerated = PROFILES[profile]
    return (
        positive_int("XP2P_RESOURCE_PLATEAU_SAMPLES", samples),
        positive_float("XP2P_RESOURCE_PLATEAU_SAMPLE_INTERVAL", interval),
        positive_float("XP2P_RESOURCE_PLATEAU_WARMUP", warmup),
        expanded,
        accelerated,
    )


def assert_owner_shutdown(env: dict, sessions: dict, aux_host) -> None:
    deadline = time.monotonic() + 15.0
    while time.monotonic() < deadline:
        client_tcp = plateau.host_peer_tcp(env["client_host"], fixture.SERVER_IP)
        aux_tcp = plateau.host_peer_tcp(aux_host, fixture.SERVER_IP)
        server_tcp = plateau.host_peer_tcp(env["server_host"], fixture.CLIENT_IP)
        server_aux_tcp = plateau.host_peer_tcp(env["server_host"], AUX_CLIENT_IP)
        if client_tcp == 0 and aux_tcp == 0 and server_tcp == 0 and server_aux_tcp == 0:
            return
        time.sleep(0.5)
    pytest.fail(
        "control connections remained after owner shutdown: "
        f"client={client_tcp}, aux={aux_tcp}, server={server_tcp}, "
        f"server_aux={server_aux_tcp}, sessions={sessions}"
    )


def collect_phase(
    payload: dict,
    name: str,
    owners: dict,
    count: int,
    interval: float,
    *,
    before_sample=None,
) -> None:
    if count <= 0:
        return
    start = len(next(iter(payload["samples"].values())))
    plateau.collect_parallel(owners, payload["samples"], count, interval, before_sample)
    payload["phases"][name] = {"start": start, "end": start + count}


def assess_phases(payload: dict) -> None:
    payload["assessments"] = {}
    for phase, bounds in payload["phases"].items():
        payload["assessments"][phase] = {}
        for owner, samples in payload["samples"].items():
            phase_samples = samples[bounds["start"] : bounds["end"]]
            limits = LIMITS
            if owner.endswith("_xp2p"):
                limits = limits | GO_LIMITS
                limits = limits | (
                    SERVER_GO_LIMITS if owner == "server_xp2p" else CLIENT_GO_LIMITS
                )
            peer_metrics = {
                key
                for sample in phase_samples
                for key in sample
                if key.startswith("tcp_peer_")
            }
            limits = limits | {key: LIMITS["tcp_peer"] for key in peer_metrics}
            payload["assessments"][phase][owner] = {}
            for metric, limit in limits.items():
                dynamic_peer = metric.startswith("tcp_peer_")
                if not dynamic_peer:
                    missing = [
                        index for index, sample in enumerate(phase_samples)
                        if metric not in sample
                    ]
                    if missing:
                        raise AssertionError(
                            f"{phase}/{owner}/{metric}: metric missing from samples {missing}"
                        )
                try:
                    result = plateau.assess(
                        [
                            sample.get(metric, 0) if dynamic_peer else sample[metric]
                            for sample in phase_samples
                        ],
                        limit,
                    )
                except AssertionError as exc:
                    raise AssertionError(f"{phase}/{owner}/{metric}: {exc}") from exc
                payload["assessments"][phase][owner][metric] = result


def assert_pids_gone(env: dict, pids: dict[str, int]) -> None:
    hosts = {
        "client_xp2p": env["client_host"],
        "client_xray": env["client_host"],
        "server_xp2p": env["server_host"],
        "server_xray": env["server_host"],
    }
    for name, pid in pids.items():
        assert hosts[name].run(f"test ! -e /proc/{pid}").rc == 0, f"{name} pid {pid} survived shutdown"
