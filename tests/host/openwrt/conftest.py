from __future__ import annotations

import pytest
from testinfra.host import Host

from tests.host.openwrt import env as openwrt_env


@pytest.fixture(scope="session")
def ipk_builder_host() -> Host:
    openwrt_env.require_ipk_builder_environment()
    return openwrt_env.get_ipk_builder_host()


@pytest.fixture(scope="session")
def openwrt_host() -> Host:
    openwrt_env.require_openwrt_environment()
    return openwrt_env.get_openwrt_host(openwrt_env.DEFAULT_OPENWRT_MACHINE)
