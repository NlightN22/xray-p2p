from __future__ import annotations

import hashlib
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
GUEST_SCRIPTS_ROOT = WORKTREE_POSIX / "tests" / "guest"
GUEST_SCRIPTS_SOURCE = REPO_ROOT / "tests" / "guest"
ALPINE_GUEST_SCRIPTS_ROOT = PurePosixPath("/tmp/xray-p2p-tests/guest")
ALPINE_DNSMASQ_INSTALLER = REPO_ROOT / "infra" / "vagrant" / "openwrt" / "dnsmasq-install-alpine.sh"
GUEST_SCRIPTS_HASH_FILE = REPO_ROOT / "build" / ".openwrt_guest_scripts.hash"
IPK_OUTPUT_DIR = REPO_ROOT / "build" / "ipk"
IPK_OUTPUT_POSIX = WORKTREE_POSIX / "build" / "ipk"
BUILDER_VAGRANT_DIR = REPO_ROOT / "infra" / "vagrant" / "debian12" / "ipk-build"
BUILDER_MACHINE = "deb12-ipk-build"
OPENWRT_VAGRANT_DIR = REPO_ROOT / "infra" / "vagrant" / "openwrt"
OPENWRT_MACHINES: tuple[str, ...] = ("openwrt-a", "openwrt-b", "openwrt-c")
ALPINE_MACHINES: tuple[str, ...] = ("c1", "c2", "c3")
DEFAULT_OPENWRT_MACHINE = OPENWRT_MACHINES[0]
TARGET_ENV_VAR = "XP2P_OPENWRT_IPK_TARGET"
DEFAULT_TARGET = "linux-amd64"

_SCRIPTS_HASH_CACHE: str | None = None


def _posix(value: PurePosixPath | Path | str) -> str:
    if isinstance(value, (PurePosixPath, Path)):
        return value.as_posix()
    return str(value)


def _compute_guest_scripts_hash() -> str:
    if not GUEST_SCRIPTS_SOURCE.exists():
        return ""
    hasher = hashlib.sha256()
    for path in sorted(GUEST_SCRIPTS_SOURCE.rglob("*")):
        if path.is_dir():
            continue
        rel = path.relative_to(GUEST_SCRIPTS_SOURCE).as_posix().encode("utf-8")
        hasher.update(rel)
        hasher.update(b"\0")
        hasher.update(path.read_bytes())
    return hasher.hexdigest()


def _read_cached_scripts_hash() -> str | None:
    if not GUEST_SCRIPTS_HASH_FILE.exists():
        return None
    data = GUEST_SCRIPTS_HASH_FILE.read_text(encoding="utf-8").strip()
    return data or None


def _write_cached_scripts_hash(value: str) -> None:
    GUEST_SCRIPTS_HASH_FILE.parent.mkdir(parents=True, exist_ok=True)
    GUEST_SCRIPTS_HASH_FILE.write_text(value, encoding="utf-8")


def _provision_guest_scripts(machine: str, destination: PurePosixPath) -> None:
    common.ensure_machine_running(OPENWRT_VAGRANT_DIR, machine)
    command = [
        "vagrant",
        "upload",
        str(GUEST_SCRIPTS_SOURCE),
        destination.as_posix(),
        machine,
    ]
    try:
        subprocess.run(command, cwd=OPENWRT_VAGRANT_DIR, check=True, text=True, capture_output=True)
    except subprocess.CalledProcessError as exc:
        raise RuntimeError(
            f"Failed to sync guest scripts into host {machine} via Vagrant.\n"
            f"STDOUT:\n{exc.stdout}\nSTDERR:\n{exc.stderr}"
        ) from exc


def _provision_file(machine: str, source: Path, destination: PurePosixPath) -> None:
    common.ensure_machine_running(OPENWRT_VAGRANT_DIR, machine)
    command = [
        "vagrant",
        "upload",
        str(source),
        destination.as_posix(),
        machine,
    ]
    try:
        subprocess.run(command, cwd=OPENWRT_VAGRANT_DIR, check=True, text=True, capture_output=True)
    except subprocess.CalledProcessError as exc:
        raise RuntimeError(
            f"Failed to upload {source} into host {machine} via Vagrant.\n"
            f"STDOUT:\n{exc.stdout}\nSTDERR:\n{exc.stderr}"
        ) from exc


def ensure_guest_scripts_synced() -> None:
    global _SCRIPTS_HASH_CACHE
    if not GUEST_SCRIPTS_SOURCE.exists():
        return
    current_hash = _compute_guest_scripts_hash()
    if _SCRIPTS_HASH_CACHE == current_hash:
        for machine in ALPINE_MACHINES:
            _provision_guest_scripts(machine, ALPINE_GUEST_SCRIPTS_ROOT)
            if ALPINE_DNSMASQ_INSTALLER.exists():
                _provision_file(machine, ALPINE_DNSMASQ_INSTALLER, PurePosixPath("/tmp/dnsmasq-install-alpine.sh"))
        return
    cached = _read_cached_scripts_hash()
    if cached == current_hash and cached is not None:
        _SCRIPTS_HASH_CACHE = current_hash
        for machine in ALPINE_MACHINES:
            _provision_guest_scripts(machine, ALPINE_GUEST_SCRIPTS_ROOT)
            if ALPINE_DNSMASQ_INSTALLER.exists():
                _provision_file(machine, ALPINE_DNSMASQ_INSTALLER, PurePosixPath("/tmp/dnsmasq-install-alpine.sh"))
        return
    require_openwrt_environment()
    for machine in OPENWRT_MACHINES:
        _provision_guest_scripts(machine, GUEST_SCRIPTS_ROOT)
    for machine in ALPINE_MACHINES:
        _provision_guest_scripts(machine, ALPINE_GUEST_SCRIPTS_ROOT)
        if ALPINE_DNSMASQ_INSTALLER.exists():
            _provision_file(machine, ALPINE_DNSMASQ_INSTALLER, PurePosixPath("/tmp/dnsmasq-install-alpine.sh"))
    if current_hash:
        _write_cached_scripts_hash(current_hash)
    _SCRIPTS_HASH_CACHE = current_hash


def run_guest_script(host: Host, relative_path: str, *args: str):
    script_path = GUEST_SCRIPTS_ROOT / relative_path
    quoted_script = shlex.quote(script_path.as_posix())
    quoted_args = " ".join(shlex.quote(str(arg)) for arg in args)
    command = f"/bin/sh {quoted_script}"
    if quoted_args:
        command = f"{command} {quoted_args}"
    return host.run(command)


def run_alpine_guest_script(host: Host, relative_path: str, *args: str):
    script_path = ALPINE_GUEST_SCRIPTS_ROOT / relative_path
    quoted_script = shlex.quote(script_path.as_posix())
    quoted_args = " ".join(shlex.quote(str(arg)) for arg in args)
    command = f"/bin/sh {quoted_script}"
    if quoted_args:
        command = f"{command} {quoted_args}"
    return host.run(command)


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


def build_ipk(host: Host, target: str) -> None:
    worktree = WORKTREE_POSIX.as_posix()
    output_dir = IPK_OUTPUT_POSIX.as_posix()
    command = (
        f"cd {shlex.quote(worktree)} && "
        f"bash ./scripts/build/build_openwrt_ipk.sh "
        f"--target {shlex.quote(target)} "
        f"--output-dir {shlex.quote(output_dir)} "
        f"--force-build"
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
    force: bool = True,
) -> PurePosixPath:
    dest = destination or PurePosixPath("/tmp/xp2p.ipk")
    staged_path = stage_ipk_on_guest(host, ipk_path, dest)
    opkg_remove(host, "xp2p", ignore_missing=True)
    _purge_xp2p_artifacts(host)
    opkg_install_local(host, staged_path)
    return staged_path


def bootstrap_xp2p_configs(host: Host) -> None:
    # Recreate default configs for both roles (needed after cleanup)
    for role, config_dir in (("client", "config-client"), ("server", "config-server")):
        result = host.run(
            f"/usr/bin/xp2p {role} install --path /etc/xp2p --config-dir {config_dir}"
        )
        if result.rc != 0:
            raise RuntimeError(
                f"xp2p {role} install failed on {host.backend.hostname} "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
    _assert_default_configs_present(host)


def cleanup_xp2p(host: Host) -> None:
    _stop_xp2p_services(host)
    host.run("OPKG_FORCE_REMOVE=1 opkg remove --autoremove xp2p >/dev/null 2>&1 || true")
    host.run("rm -f /tmp/xp2p-client.log /tmp/xp2p-server.log /tmp/xp2p.ipk >/dev/null 2>&1 || true")
    host.run("rm -f /etc/xp2p/dns-forward-state.json >/dev/null 2>&1 || true")
    _purge_xp2p_artifacts(host)


def _purge_xp2p_artifacts(host: Host) -> None:
    host.run("rm -rf /etc/xp2p /usr/bin/xp2p /usr/lib/xp2p >/dev/null 2>&1 || true")


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


def _stop_xp2p_services(host: Host) -> None:
    for cmd in (
        "xp2p client service stop --quiet",
        "xp2p server service stop --quiet",
        "/etc/init.d/xp2p stop",
        "/etc/init.d/xp2p-client stop",
        "/etc/init.d/xp2p-server stop",
    ):
        host.run(f"{cmd} >/dev/null 2>&1 || true")


def _kill_port_listeners(host: Host, port: str) -> None:
    kill_cmd = (
        "pids=$(netstat -lpn 2>/dev/null | grep ':%s ' | awk '{print $7}' | cut -d/ -f1 | tr -d \"-\" ); "
        "for p in $pids; do [ -n \"$p\" ] && kill -9 \"$p\" >/dev/null 2>&1 || true; done"
    ) % shlex.quote(port)
    host.run(f"sh -c {shlex.quote(kill_cmd)}")


def _netstat_snapshot(host: Host) -> str:
    result = host.run(
        "netstat -lpn 2>/dev/null | egrep '62022|62023|52080|52180|51080|51180' || true"
    )
    return result.stdout or ""


def _read_file_safe(host: Host, path: PurePosixPath | Path | str) -> str:
    target = _posix(path)
    result = host.run(f"cat {shlex.quote(target)} 2>/dev/null || true")
    if result.rc != 0:
        return ""
    return result.stdout or ""


def _assert_default_configs_present(host: Host) -> None:
    required = [
        "/etc/xp2p/config-client/inbounds.json",
        "/etc/xp2p/config-client/outbounds.json",
        "/etc/xp2p/config-client/routing.json",
        "/etc/xp2p/config-client/logs.json",
        "/etc/xp2p/config-server/inbounds.json",
        "/etc/xp2p/config-server/outbounds.json",
        "/etc/xp2p/config-server/routing.json",
        "/etc/xp2p/config-server/logs.json",
    ]
    missing: list[str] = []
    for path in required:
        if host.run(f"test -f {shlex.quote(path)}").rc != 0:
            missing.append(path)
    if missing:
        raise RuntimeError(
            f"xp2p default configs are missing on {host.backend.hostname}: {', '.join(missing)}"
        )


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
    _stop_xp2p_services(host)
    for port in ("62022", "62023", "52080", "52180", "51080", "51180"):
        host.run(f"fuser -k {port}/tcp >/dev/null 2>&1 || true")
        host.run(f"fuser -k {port}/udp >/dev/null 2>&1 || true")
        _kill_port_listeners(host, port)
    host.run("pkill -f 'xp2p server run' >/dev/null 2>&1 || true")
    host.run("pkill -f 'xp2p client run' >/dev/null 2>&1 || true")
    host.run("pkill -f '/etc/xp2p/bin/xray' >/dev/null 2>&1 || true")
    host.run("ps w | grep '/etc/xp2p/bin/xray' | grep -v grep | awk '{print $1}' | xargs -r kill -9 >/dev/null 2>&1 || true")
    host.run("ps w | grep '/usr/bin/xp2p' | grep -v grep | awk '{print $1}' | xargs -r kill -9 >/dev/null 2>&1 || true")
    host.run(f"mkdir -p {shlex.quote(log_dir)}")
    netstat_before = _netstat_snapshot(host)
    logs_path = PurePosixPath(install_dir) / config_dir / "logs.json"
    logs_config = _read_file_safe(host, logs_path)
    start_cmd = (
        f"setsid /usr/bin/xp2p {role} run "
        f"--path {shlex.quote(install_path)} "
        f"--config-dir {shlex.quote(config_dir)} "
        f"--auto-install "
        f"--xray-log-file {shlex.quote(log_file)} "
        f"--quiet >/tmp/xp2p-{role}-run.log 2>&1 & echo $!"
    )
    last_log = ""
    pid_value: str | None = None
    for attempt in range(2):
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
        if alive.rc == 0:
            break
        log_read = host.run(f"cat /tmp/xp2p-{role}-run.log 2>/dev/null || true")
        last_log = log_read.stdout or ""
        host.run(f"pkill -f 'xp2p {role} run' >/dev/null 2>&1 || true")
        host.run("pkill -f '/etc/xp2p/bin/xray' >/dev/null 2>&1 || true")
        time.sleep(1.0)
    else:
        raise RuntimeError(
            "xp2p {role} run exited prematurely (pid {pid}).\n"
            "Log output:\n{log}\n"
            "netstat before start:\n{netstat}\n"
            "logs.json ({logs_path}):\n{logs_cfg}".format(
                role=role,
                pid=pid_value,
                log=last_log,
                netstat=netstat_before,
                logs_path=logs_path.as_posix(),
                logs_cfg=logs_config,
            )
        )
    try:
        yield {"pid": int(pid_value), "log": log_file}
    finally:
        host.run(f"kill {pid_value} >/dev/null 2>&1 || true")
