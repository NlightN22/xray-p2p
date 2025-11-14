from __future__ import annotations

import shlex
from pathlib import Path, PurePosixPath
from typing import Callable

from testinfra.backend.base import CommandResult
from testinfra.host import Host

from tests.host import common

REPO_ROOT = common.REPO_ROOT
VAGRANT_DIR = REPO_ROOT / "infra" / "vagrant" / "debian12" / "deb-test"
MACHINE_IDS: tuple[str, ...] = (
    "deb-test-a",
    "deb-test-b",
    "deb-test-c",
)
WORK_TREE = PurePosixPath("/srv/xray-p2p")
INSTALL_PATH = PurePosixPath("/usr/bin/xp2p")
GUEST_SCRIPTS_ROOT = WORK_TREE / "tests" / "guest"

_VERSION_CACHE: dict[str, dict[str, str]] = {}


def require_vagrant_environment() -> None:
    common.require_vagrant_environment(VAGRANT_DIR)


def ensure_machine_running(machine: str) -> None:
    common.ensure_machine_running(VAGRANT_DIR, machine)


def get_ssh_host(machine: str) -> Host:
    return common.get_ssh_host(VAGRANT_DIR, machine)


def _run_shell(host: Host, script: str) -> CommandResult:
    quoted = shlex.quote(script)
    return host.run(f"bash -lc {quoted}")


def run_guest_script(host: Host, relative_path: str, *args: str) -> CommandResult:
    script_path = GUEST_SCRIPTS_ROOT / relative_path
    quoted_script = shlex.quote(str(script_path))
    quoted_args = " ".join(shlex.quote(str(arg)) for arg in args)
    return host.run(f"bash {quoted_script} {quoted_args}".strip())


def _install_marker(marker: str, output: str | None) -> str | None:
    for line in (output or "").splitlines():
        line = line.strip()
        if line.startswith(marker):
            return line[len(marker) :].strip()
    return None


def ensure_xp2p_installed(machine: str, host: Host) -> dict[str, str]:
    cached = _VERSION_CACHE.get(machine)
    if cached:
        return cached

    result = run_guest_script(host, "scripts/linux/install_xp2p.sh")
    if result.rc != 0:
        raise RuntimeError(
            "Failed to build and install xp2p on guest "
            f"{machine} (exit {result.rc}).\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )

    source_version = _install_marker("__XP2P_SOURCE_VERSION__=", result.stdout)
    installed_version = _install_marker("__XP2P_INSTALLED_VERSION__=", result.stdout)
    if not source_version or not installed_version:
        raise RuntimeError(
            "xp2p install script did not emit expected markers.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )

    versions = {"source": source_version, "installed": installed_version}
    _VERSION_CACHE[machine] = versions
    return versions


def run_xp2p(host: Host, *args: str) -> CommandResult:
    quoted_args = " ".join(shlex.quote(arg) for arg in args)
    return host.run(f"{INSTALL_PATH} {quoted_args}")


def machine_host_factory() -> Callable[[str], Host]:
    cache: dict[str, Host] = {}

    def _get(machine: str) -> Host:
        if machine not in MACHINE_IDS:
            raise ValueError(f"Unknown machine id: {machine}")
        if machine not in cache:
            ensure_machine_running(machine)
            cache[machine] = get_ssh_host(machine)
        return cache[machine]

    return _get
