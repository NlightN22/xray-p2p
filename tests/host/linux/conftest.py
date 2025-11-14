from __future__ import annotations

from typing import Callable

import pytest
from testinfra.host import Host

from . import env as linux_env


@pytest.fixture(scope="session")
def linux_host_factory() -> Callable[[str], Host]:
    linux_env.require_vagrant_environment()
    return linux_env.machine_host_factory()


@pytest.fixture(scope="session")
def xp2p_linux_versions(linux_host_factory) -> dict[str, dict[str, str]]:
    versions: dict[str, dict[str, str]] = {}
    for machine in linux_env.MACHINE_IDS:
        host = linux_host_factory(machine)
        versions[machine] = linux_env.ensure_xp2p_installed(machine, host)
    return versions
