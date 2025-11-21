from __future__ import annotations

import os
import subprocess
from contextlib import contextmanager
from pathlib import Path, PurePosixPath
import shlex
import time
from typing import Callable

from testinfra.host import Host

from tests.host import common

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


def _posix(value: PurePosixPath | Path | str) -> str:
    if isinstance(value, (PurePosixPath, Path)):
        return value.as_posix()
    return str(value)


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


def host_factory() -> Callable[[str], Host]:
    cache: dict[str, Host] = {}

    def _get(machine: str) -> Host:
        if machine not in OPENWRT_MACHINES:
            raise ValueError(f"Unknown OpenWrt machine id: {machine}")
        if machine not in cache:
            require_openwrt_environment()
            cache[machine] = get_openwrt_host(machine)
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
    remote_source = PurePosixPath("/tmp/build-openwrt") / ipk_path.name
    copy_command = f"cp {shlex.quote(remote_source.as_posix())} {shlex.quote(target_path.as_posix())}"
    result = host.run(copy_command)
    if result.rc != 0:
        raise RuntimeError(
            "Failed to copy ipk from /tmp/build-openwrt.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return target_path


def install_ipk_on_host(
    host: Host,
    ipk_path: Path,
    *,
    destination: PurePosixPath | None = None,
    force: bool = False,
) -> PurePosixPath:
    dest = destination or PurePosixPath("/tmp/xp2p.ipk")
    if not force:
        binary_check = host.run("test -x /usr/bin/xp2p")
        status = host.run("opkg status xp2p")
        if binary_check.rc == 0 and status.rc == 0:
            return dest
    staged_path = stage_ipk_on_guest(host, ipk_path, dest)
    opkg_remove(host, "xp2p", ignore_missing=True)
    opkg_install_local(host, staged_path)
    return staged_path


def opkg_remove(host: Host, package: str, ignore_missing: bool = True) -> None:
    status = host.run(f"opkg status {shlex.quote(package)}")
    if status.rc != 0:
        if ignore_missing:
            return
        raise RuntimeError(
            f"Package {package} is not installed.\nSTDOUT:\n{status.stdout}\nSTDERR:\n{status.stderr}"
        )
    result = host.run(f"opkg remove {shlex.quote(package)}")
    if result.rc != 0:
        raise RuntimeError(
            f"Failed to remove package {package} "
            f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def opkg_install_local(host: Host, path: PurePosixPath) -> None:
    result = host.run(
        f"opkg install --force-reinstall {shlex.quote(path.as_posix())}"
    )
    if result.rc != 0:
        raise RuntimeError(
            f"Failed to install ipk {path} "
            f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def run_xp2p(host: Host, *args: str):
    quoted_args = " ".join(shlex.quote(arg) for arg in args)
    command = "/usr/bin/xp2p"
    if quoted_args:
        command = f"{command} {quoted_args}"
    return host.run(command)


def resolve_target_from_env() -> str:
    return os.environ.get(TARGET_ENV_VAR, DEFAULT_TARGET)


@contextmanager
def xp2p_run_session(
    host: Host,
    role: str,
    install_dir: str | Path | PurePosixPath,
    config_dir: str,
    log_path: str | Path | PurePosixPath,
):
    if role not in {"server", "client"}:
        raise ValueError(f"Unsupported role: {role}")
    install_path = _posix(install_dir)
    log_file = _posix(log_path)
    log_dir = str(PurePosixPath(log_file).parent)
    host.run(f"mkdir -p {shlex.quote(log_dir)}")
    start_cmd = (
        f"setsid /usr/bin/xp2p {role} run "
        f"--path {shlex.quote(install_path)} "
        f"--config-dir {shlex.quote(config_dir)} "
        f"--auto-install "
        f"--xray-log-file {shlex.quote(log_file)} "
        f"--quiet >/tmp/xp2p-{role}-run.log 2>&1 & echo $!"
    )
    result = host.run(f"sh -c {shlex.quote(start_cmd)}")
    if result.rc != 0:
        raise RuntimeError(
            f"Failed to start xp2p {role} run.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    pid_line = (result.stdout or "").strip().splitlines()
    if not pid_line:
        raise RuntimeError("xp2p run did not output PID")
    pid_value = pid_line[-1].strip()
    time.sleep(1)
    alive = host.run(f"kill -0 {pid_value} >/dev/null 2>&1")
    if alive.rc != 0:
        raise RuntimeError(f"xp2p {role} run exited prematurely (pid {pid_value}).")
    try:
        yield {"pid": int(pid_value), "log": log_file}
    finally:
        host.run(f"kill {pid_value} >/dev/null 2>&1 || true")
