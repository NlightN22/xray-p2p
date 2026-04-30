from __future__ import annotations

import subprocess
from pathlib import Path, PurePosixPath
from typing import Callable

from testinfra.host import Host

from tests.host import common

REPO_ROOT = common.REPO_ROOT
WORKTREE_POSIX = PurePosixPath("/srv/xray-p2p")

BUILDER_VAGRANT_DIR = REPO_ROOT / "infra" / "vagrant" / "debian12" / "ipk-build"
BUILDER_MACHINE = "deb12-ipk-build"

OPENWRT_VAGRANT_DIR = REPO_ROOT / "infra" / "vagrant" / "openwrt"
OPENWRT_MACHINES: tuple[str, ...] = ("openwrt-a", "openwrt-b", "openwrt-c")
ALPINE_MACHINES: tuple[str, ...] = ("c1", "c2", "c3")
DEFAULT_OPENWRT_MACHINE = OPENWRT_MACHINES[0]


def require_ipk_builder_environment() -> None:
    common.require_vagrant_environment(BUILDER_VAGRANT_DIR)


def require_openwrt_environment() -> None:
    common.require_vagrant_environment(OPENWRT_VAGRANT_DIR)


def get_ipk_builder_host() -> Host:
    common.ensure_machine_running(BUILDER_VAGRANT_DIR, BUILDER_MACHINE)
    return common.get_ssh_host(BUILDER_VAGRANT_DIR, BUILDER_MACHINE)


def get_openwrt_host(machine: str) -> Host:
    if machine not in OPENWRT_MACHINES:
        raise ValueError(f"Unknown OpenWrt machine id: {machine}")
    common.ensure_machine_running(OPENWRT_VAGRANT_DIR, machine)
    return common.get_ssh_host(OPENWRT_VAGRANT_DIR, machine)


def get_alpine_host(machine: str) -> Host:
    if machine not in ALPINE_MACHINES:
        raise ValueError(f"Unknown Alpine machine id: {machine}")
    common.ensure_machine_running(OPENWRT_VAGRANT_DIR, machine)
    return common.get_ssh_host(OPENWRT_VAGRANT_DIR, machine)


def host_factory() -> Callable[[str], Host]:
    cache: dict[str, Host] = {}

    def _get(machine: str) -> Host:
        if machine not in OPENWRT_MACHINES:
            raise ValueError(f"Unknown OpenWrt machine id: {machine}")
        if machine not in cache:
            from tests.host.openwrt._sh import ensure_guest_scripts_synced

            require_openwrt_environment()
            ensure_guest_scripts_synced()
            cache[machine] = get_openwrt_host(machine)
        return cache[machine]

    return _get


def alpine_host_factory() -> Callable[[str], Host]:
    cache: dict[str, Host] = {}

    def _get(machine: str) -> Host:
        if machine not in ALPINE_MACHINES:
            raise ValueError(f"Unknown Alpine machine id: {machine}")
        if machine not in cache:
            from tests.host.openwrt._sh import ensure_guest_scripts_synced

            require_openwrt_environment()
            ensure_guest_scripts_synced()
            cache[machine] = get_alpine_host(machine)
        return cache[machine]

    return _get


def sync_build_output(machine: str = DEFAULT_OPENWRT_MACHINE) -> None:
    if machine not in OPENWRT_MACHINES:
        raise ValueError(f"Unknown OpenWrt machine id: {machine}")
    require_openwrt_environment()
    command = ["vagrant", "provision", machine, "--provision-with", "file"]
    try:
        subprocess.run(command, cwd=OPENWRT_VAGRANT_DIR, check=True, text=True, capture_output=True)
    except subprocess.CalledProcessError as exc:
        raise RuntimeError(
            "Failed to sync build/ipk into OpenWrt guest via Vagrant file provisioner:\n"
            f"STDOUT:\n{exc.stdout}\nSTDERR:\n{exc.stderr}"
        ) from exc
