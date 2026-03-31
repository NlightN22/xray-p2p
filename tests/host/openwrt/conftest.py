from __future__ import annotations

import os
import time

import pytest
from testinfra.host import Host

from tests.host.openwrt import env as openwrt_env


@pytest.fixture(scope="session")
def ipk_builder_host() -> Host:
    openwrt_env.require_ipk_builder_environment()
    return openwrt_env.get_ipk_builder_host()


@pytest.fixture(scope="session")
def openwrt_host_factory():
    openwrt_env.require_openwrt_environment()
    return openwrt_env.host_factory()


@pytest.fixture(scope="session")
def alpine_host_factory():
    openwrt_env.require_openwrt_environment()
    return openwrt_env.alpine_host_factory()


@pytest.fixture(scope="session")
def openwrt_host(openwrt_host_factory) -> Host:
    return openwrt_host_factory(openwrt_env.DEFAULT_OPENWRT_MACHINE)


@pytest.fixture(scope="session")
def openwrt_server_host(openwrt_host_factory) -> Host:
    return openwrt_host_factory(openwrt_env.OPENWRT_MACHINES[0])


@pytest.fixture(scope="session")
def openwrt_client_host(openwrt_host_factory) -> Host:
    return openwrt_host_factory(openwrt_env.OPENWRT_MACHINES[1])


@pytest.fixture(scope="session")
def alpine_c1_host(alpine_host_factory) -> Host:
    return alpine_host_factory(openwrt_env.ALPINE_MACHINES[0])


@pytest.fixture(scope="session")
def alpine_c2_host(alpine_host_factory) -> Host:
    return alpine_host_factory(openwrt_env.ALPINE_MACHINES[1])


@pytest.fixture(scope="session")
def openwrt_ipk_target() -> str:
    return openwrt_env.resolve_target_from_env()


@pytest.fixture(scope="session")
def xp2p_openwrt_ipk(openwrt_ipk_target):
    openwrt_env.IPK_OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    force_build = os.environ.get("XP2P_OPENWRT_FORCE_BUILD", "").strip().lower() in {"1", "true", "yes"}
    artifact = openwrt_env.latest_local_ipk()
    if not force_build and artifact:
        openwrt_env.ensure_packages_index_present()
        print(f"TIMING: openwrt ipk build skipped (cached {artifact.name})")
    else:
        openwrt_env.require_ipk_builder_environment()
        ipk_builder_host = openwrt_env.get_ipk_builder_host()
        build_start = time.perf_counter()
        openwrt_env.build_ipk(ipk_builder_host, openwrt_ipk_target)
        build_elapsed = time.perf_counter() - build_start
        print(f"TIMING: openwrt ipk build: {build_elapsed:.2f}s")
        artifact = openwrt_env.latest_local_ipk()
        assert artifact, "Expected build/ipk to contain a freshly built xp2p ipk"
        openwrt_env.ensure_packages_index_present()
    skip_sync = os.environ.get("XP2P_OPENWRT_SKIP_IPK_SYNC", "").strip().lower() in {"1", "true", "yes"}
    if not skip_sync:
        sync_start = time.perf_counter()
        for machine in openwrt_env.OPENWRT_MACHINES:
            openwrt_env.sync_build_output(machine)
        sync_elapsed = time.perf_counter() - sync_start
        print(f"TIMING: openwrt build output sync: {sync_elapsed:.2f}s")
    else:
        print("TIMING: openwrt build output sync skipped (env XP2P_OPENWRT_SKIP_IPK_SYNC)")
    return artifact
