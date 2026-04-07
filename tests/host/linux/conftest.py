from __future__ import annotations

from pathlib import PurePosixPath
import shlex
from collections.abc import Mapping
from typing import Callable

import pytest
from testinfra.host import Host

from . import _helpers as helpers
from . import env as linux_env


@pytest.fixture(scope="session")
def linux_host_factory() -> Callable[[str], Host]:
    linux_env.require_vagrant_environment()
    return linux_env.machine_host_factory()


@pytest.fixture(scope="session", autouse=True)
def xp2p_linux_install_session(linux_host_factory) -> None:
    for machine in linux_env.MACHINE_IDS:
        host = linux_host_factory(machine)
        linux_env.ensure_xp2p_installed(machine, host)
        linux_env.run_xp2p(host, "client", "service", "stop")
        linux_env.run_xp2p(host, "server", "service", "stop")


class _LazyLinuxVersions(Mapping[str, dict[str, str]]):
    def __init__(self, host_factory: Callable[[str], Host]) -> None:
        self._host_factory = host_factory
        self._cache: dict[str, dict[str, str]] = {}

    def __getitem__(self, key: str) -> dict[str, str]:
        if key not in linux_env.MACHINE_IDS:
            raise KeyError(key)
        if key not in self._cache:
            host = self._host_factory(key)
            self._cache[key] = linux_env.ensure_xp2p_installed(key, host)
            linux_env.run_xp2p(host, "client", "service", "stop")
            linux_env.run_xp2p(host, "server", "service", "stop")
        return self._cache[key]

    def __iter__(self):
        return iter(linux_env.MACHINE_IDS)

    def __len__(self) -> int:
        return len(linux_env.MACHINE_IDS)


@pytest.fixture(scope="session")
def xp2p_linux_versions(linux_host_factory) -> Mapping[str, dict[str, str]]:
    return _LazyLinuxVersions(linux_host_factory)


@pytest.fixture(scope="session")
def client_host(linux_host_factory) -> Host:
    return linux_host_factory(linux_env.DEFAULT_CLIENT)


@pytest.fixture(scope="session")
def server_host(linux_host_factory) -> Host:
    return linux_host_factory(linux_env.DEFAULT_SERVER)


@pytest.fixture(scope="session")
def aux_host(linux_host_factory) -> Host:
    return linux_host_factory(linux_env.DEFAULT_AUX)



def _xp2p_runner(host: Host):
    def _runner(*args: str, check: bool = False):
        cmd = list(args)
        pending_targets = {
            ("client", "list"),
            ("client", "forward", "list"),
            ("client", "redirect", "list"),
            ("client", "reverse"),
            ("client", "reverse", "list"),
            ("server", "forward", "list"),
            ("server", "redirect", "list"),
            ("server", "reverse"),
            ("server", "reverse", "list"),
            ("server", "user", "list"),
            ("server", "cert", "state"),
        }
        if "--pending" not in cmd and "-y" not in cmd:
            for target in pending_targets:
                if tuple(cmd[: len(target)]) == target:
                    cmd.append("--pending")
                    break
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


@pytest.fixture(autouse=True)
def xp2p_full_cleanup(request, linux_host_factory):
    module_name = request.fspath.basename if request.fspath else ""
    needed: set[str] = set()
    if "client_host" in request.fixturenames or "server_host" in request.fixturenames:
        needed.update({linux_env.DEFAULT_CLIENT, linux_env.DEFAULT_SERVER})
    if "aux_host" in request.fixturenames:
        needed.add(linux_env.DEFAULT_AUX)
    if "linux_host_factory" in request.fixturenames:
        needed.update({linux_env.DEFAULT_CLIENT, linux_env.DEFAULT_SERVER})
        if module_name == "test_tunnel_BC_to_A.py":
            needed.add(linux_env.DEFAULT_AUX)

    hosts = [linux_host_factory(machine) for machine in sorted(needed)]

    for host in hosts:
        _cleanup_host(host)
    yield


@pytest.fixture(scope="session", autouse=True)
def xp2p_session_cleanup(linux_host_factory):
    yield
    hosts = [linux_host_factory(machine) for machine in linux_env.MACHINE_IDS]
    for host in hosts:
        _cleanup_host(host)


def _cleanup_host(host: Host) -> None:
    runner = _xp2p_runner(host)
    runner(
        "client",
        "remove",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
        "--all",
        "--ignore-missing",
    )
    runner(
        "server",
        "remove",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.SERVER_CONFIG_DIR_NAME,
        "--ignore-missing",
    )
    runner("client", "service", "stop")
    runner("server", "service", "stop")
    linux_env.kill_xp2p_processes(host)
    root = helpers.CONFIG_ROOT.as_posix()
    quoted_root = shlex.quote(root)
    host.run(
        "sudo -n /bin/sh -c "
        "'for path in \"$1\".bak-*; do [ -e \"$path\" ] || continue; rm -rf \"$path\"; done' "
        f"-- {quoted_root}"
    )
    bundle_artifacts = linux_env.WORK_TREE / "build" / "artifacts" / "bundle"
    cleanup_paths = [
        helpers.CONFIG_ROOT / ".apply",
        helpers.CLIENT_CONFIG_FILE,
        helpers.SERVER_CONFIG_FILE,
        helpers.CLIENT_APPLIED_STATE_FILE,
        helpers.SERVER_APPLIED_STATE_FILE,
        helpers.CLIENT_HEARTBEAT_STATE_FILE,
        helpers.SERVER_HEARTBEAT_STATE_FILE,
        helpers.CLIENT_CONFIG_DIR / "inbounds.json",
        helpers.SERVER_CONFIG_DIR / "inbounds.json",
        *helpers.SERVICE_LOG_FILES,
        helpers.CONFIG_ROOT / "bundle-marker.txt",
        bundle_artifacts,
        PurePosixPath("/tmp/xp2p-client-deploy.log"),
        PurePosixPath("/tmp/xp2p-server-deploy.log"),
    ]
    quoted_paths = " ".join(shlex.quote(path.as_posix()) for path in cleanup_paths)
    host.run(f"sudo -n /bin/sh -c 'rm -rf -- \"$@\"' -- {quoted_paths}")
