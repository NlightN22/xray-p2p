from __future__ import annotations

import os
from pathlib import Path, PurePosixPath
import shlex

from testinfra.host import Host

from tests.host import common
from tests.host.linux import env as linux_env

REPO_ROOT = common.REPO_ROOT
WORKTREE_POSIX = PurePosixPath("/srv/xray-p2p")
IPK_OUTPUT_DIR = REPO_ROOT / "build" / "ipk"
IPK_OUTPUT_POSIX = WORKTREE_POSIX / "build" / "ipk"
BUILDER_VAGRANT_DIR = REPO_ROOT / "infra" / "vagrant" / "debian12" / "ipk-build"
BUILDER_MACHINE = "deb12-ipk-build"
OPENWRT_VAGRANT_DIR = REPO_ROOT / "infra" / "vagrant" / "openwrt"
OPENWRT_MACHINES: tuple[str, ...] = ("openwrt-a", "openwrt-b", "openwrt-c")
DEFAULT_OPENWRT_MACHINE = OPENWRT_MACHINES[0]
TARGET_ENV_VAR = "XP2P_OPENWRT_IPK_TARGET"
DEFAULT_TARGET = "linux-amd64"


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


def _run_script(host: Host, relative_path: str, *args: str):
    return linux_env.run_guest_script(host, relative_path, *args)


def build_ipk(host: Host, target: str) -> None:
    worktree = WORKTREE_POSIX.as_posix()
    output_dir = IPK_OUTPUT_POSIX.as_posix()
    command = (
        f"cd {shlex.quote(worktree)} && "
        f"./scripts/build/build_openwrt_ipk.sh "
        f"--target {shlex.quote(target)} "
        f"--output-dir {shlex.quote(output_dir)}"
    )
    result = host.run(f"bash -lc {shlex.quote(command)}")
    if result.rc != 0:
        raise RuntimeError(
            "OpenWrt build script failed "
            f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def latest_local_ipk() -> Path | None:
    if not IPK_OUTPUT_DIR.exists():
        return None
    candidates = list(IPK_OUTPUT_DIR.glob("xp2p_*.ipk"))
    if not candidates:
        return None
    candidates.sort(key=lambda path: path.stat().st_mtime)
    return candidates[-1]


def ensure_packages_index_present() -> None:
    packages = IPK_OUTPUT_DIR / "Packages"
    packages_gz = IPK_OUTPUT_DIR / "Packages.gz"
    if not packages.exists():
        raise AssertionError(f"Expected Packages file at {packages}")
    if not packages_gz.exists():
        raise AssertionError(f"Expected Packages.gz file at {packages_gz}")


def stage_ipk_on_guest(host: Host, ipk_path: Path, destination: PurePosixPath | None = None) -> PurePosixPath:
    target_path = destination or PurePosixPath("/tmp/xp2p.ipk")
    host.put_file(local_path=str(ipk_path), remote_path=target_path.as_posix())
    return target_path


def opkg_remove(host: Host, package: str, ignore_missing: bool = True) -> None:
    args = ["--package", package]
    if ignore_missing:
        args.append("--ignore-missing")
    result = _run_script(host, "scripts/openwrt/opkg_remove.sh", *args)
    if result.rc != 0:
        raise RuntimeError(
            f"Failed to remove package {package} "
            f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def opkg_install_local(host: Host, path: PurePosixPath) -> None:
    result = _run_script(host, "scripts/openwrt/opkg_install_local.sh", "--path", path.as_posix())
    if result.rc != 0:
        raise RuntimeError(
            f"Failed to install ipk {path} "
            f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def run_xp2p(host: Host, *args: str):
    return _run_script(host, "scripts/openwrt/run_xp2p.sh", *args)


def resolve_target_from_env() -> str:
    return os.environ.get(TARGET_ENV_VAR, DEFAULT_TARGET)
