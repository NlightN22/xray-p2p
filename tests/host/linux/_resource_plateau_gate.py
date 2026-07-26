from __future__ import annotations

import os
import time

import pytest

from tests.host.linux import _resource_plateau as plateau
from tests.host.linux.flows import tunnel_b_to_a_fixture as fixture

QUICK_SAMPLES = 24
NIGHTLY_SAMPLES = 720
SAMPLE_INTERVAL_SECONDS = 5.0
WARMUP_SECONDS = 30.0
AUX_CLIENT_IP = "10.62.10.13"
LIMITS = {
    "rss_kib": plateau.PlateauLimit(32 * 1024, 256),
    "threads": plateau.PlateauLimit(8, 0.1),
    "fd": plateau.PlateauLimit(12, 0.1),
    "socket_fd": plateau.PlateauLimit(8, 0.1),
    "pipe_fd": plateau.PlateauLimit(8, 0.1),
    "anon_fd": plateau.PlateauLimit(8, 0.1),
    "tcp_total": plateau.PlateauLimit(8, 0.1),
    "tcp_estab": plateau.PlateauLimit(8, 0.1),
    "tcp_peer": plateau.PlateauLimit(8, 0.1),
    "cgroup_memory": plateau.PlateauLimit(64 * 1024 * 1024, 512 * 1024),
}
GO_LIMITS = {
    "go_heap_alloc": plateau.PlateauLimit(32 * 1024 * 1024, 256 * 1024),
    "go_heap_sys": plateau.PlateauLimit(32 * 1024 * 1024, 256 * 1024),
    "go_goroutines": plateau.PlateauLimit(16, 0.1),
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


def collect_phase(payload: dict, name: str, owners: dict, count: int, interval: float) -> None:
    if count <= 0:
        return
    start = len(next(iter(payload["samples"].values())))
    plateau.collect_parallel(owners, payload["samples"], count, interval)
    payload["phases"][name] = {"start": start, "end": start + count}


def assert_pids_gone(env: dict, pids: dict[str, int]) -> None:
    hosts = {
        "client_xp2p": env["client_host"],
        "client_xray": env["client_host"],
        "server_xp2p": env["server_host"],
        "server_xray": env["server_host"],
    }
    for name, pid in pids.items():
        assert hosts[name].run(f"test ! -e /proc/{pid}").rc == 0, f"{name} pid {pid} survived shutdown"
