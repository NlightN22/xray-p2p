from __future__ import annotations

from typing import Callable

import pytest
from testinfra.host import Host

from . import env as linux_env


@pytest.fixture(scope="session")
def linux_host_factory() -> Callable[[str], Host]:
    linux_env.require_vagrant_environment()
    return linux_env.machine_host_factory()
