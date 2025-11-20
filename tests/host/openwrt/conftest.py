from __future__ import annotations

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
def openwrt_host(openwrt_host_factory) -> Host:
    return openwrt_host_factory(openwrt_env.DEFAULT_OPENWRT_MACHINE)


@pytest.fixture(scope="session")
def openwrt_server_host(openwrt_host_factory) -> Host:
    return openwrt_host_factory(openwrt_env.OPENWRT_MACHINES[0])


@pytest.fixture(scope="session")
def openwrt_client_host(openwrt_host_factory) -> Host:
    return openwrt_host_factory(openwrt_env.OPENWRT_MACHINES[1])


@pytest.fixture(scope="session")
def openwrt_ipk_target() -> str:
    return openwrt_env.resolve_target_from_env()


@pytest.fixture(scope="session")
def xp2p_openwrt_ipk(ipk_builder_host, openwrt_ipk_target):
    openwrt_env.IPK_OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    openwrt_env.build_ipk(ipk_builder_host, openwrt_ipk_target)
    artifact = openwrt_env.latest_local_ipk()
    assert artifact, "Expected build/ipk to contain a freshly built xp2p ipk"
    openwrt_env.ensure_packages_index_present()
    return artifact
