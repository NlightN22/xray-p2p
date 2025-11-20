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


@pytest.fixture(scope="session")
def client_host(linux_host_factory, xp2p_linux_versions) -> Host:
    return linux_host_factory(linux_env.DEFAULT_CLIENT)


@pytest.fixture(scope="session")
def server_host(linux_host_factory, xp2p_linux_versions) -> Host:
    return linux_host_factory(linux_env.DEFAULT_SERVER)


@pytest.fixture(scope="session")
def aux_host(linux_host_factory, xp2p_linux_versions) -> Host:
    return linux_host_factory(linux_env.DEFAULT_AUX)



def _xp2p_runner(host: Host):
    def _runner(*args: str, check: bool = False):
        cmd = list(args)
        if len(cmd) >= 2 and cmd[0] in {"client", "server"} and cmd[1] == "remove":
            if not any(arg == "--quiet" for arg in cmd):
                cmd.append("--quiet")
        result = linux_env.run_xp2p(host, *cmd)
        if check and result.rc != 0:
            pytest.fail(
                "xp2p command failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        return result

    return _runner


@pytest.fixture
def xp2p_client_runner(client_host: Host):
    return _xp2p_runner(client_host)


@pytest.fixture
def xp2p_server_runner(server_host: Host):
    return _xp2p_runner(server_host)
