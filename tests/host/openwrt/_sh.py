from __future__ import annotations

import hashlib
import os
import shlex
import subprocess
import time
from pathlib import Path, PurePosixPath

from testinfra.host import Host

from tests.host import common
from tests.host.openwrt._vagrant import (
    ALPINE_MACHINES,
    OPENWRT_MACHINES,
    OPENWRT_VAGRANT_DIR,
    REPO_ROOT,
    require_openwrt_environment,
)

GUEST_SCRIPTS_ROOT = PurePosixPath("/tmp/xray-p2p-tests/guest")
ALPINE_GUEST_SCRIPTS_ROOT = PurePosixPath("/tmp/xray-p2p-tests/guest")
GUEST_SCRIPTS_SOURCE = REPO_ROOT / "tests" / "guest"
ALPINE_DNSMASQ_INSTALLER = REPO_ROOT / "infra" / "vagrant" / "openwrt" / "dnsmasq-install-alpine.sh"
GUEST_SCRIPTS_HASH_FILE = REPO_ROOT / "build" / ".openwrt_guest_scripts.hash"

_SCRIPTS_HASH_CACHE: str | None = None
_SCRIPTS_SYNCED: bool = False

__all__ = [
    "ALPINE_DNSMASQ_INSTALLER",
    "ALPINE_GUEST_SCRIPTS_ROOT",
    "GUEST_SCRIPTS_HASH_FILE",
    "GUEST_SCRIPTS_ROOT",
    "GUEST_SCRIPTS_SOURCE",
    "_SCRIPTS_HASH_CACHE",
    "_SCRIPTS_SYNCED",
    "_compute_guest_scripts_hash",
    "_provision_file",
    "_provision_guest_scripts",
    "_read_cached_scripts_hash",
    "_write_cached_scripts_hash",
    "ensure_guest_scripts_synced",
    "run_alpine_guest_script",
    "run_guest_script",
    "stop_process",
]


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
    command = ["vagrant", "upload", str(GUEST_SCRIPTS_SOURCE), destination.as_posix(), machine]
    try:
        subprocess.run(command, cwd=OPENWRT_VAGRANT_DIR, check=True, text=True, capture_output=True)
    except subprocess.CalledProcessError as exc:
        raise RuntimeError(
            f"Failed to sync guest scripts into host {machine} via Vagrant.\n"
            f"STDOUT:\n{exc.stdout}\nSTDERR:\n{exc.stderr}"
        ) from exc


def _provision_file(machine: str, source: Path, destination: PurePosixPath) -> None:
    common.ensure_machine_running(OPENWRT_VAGRANT_DIR, machine)
    command = ["vagrant", "upload", str(source), destination.as_posix(), machine]
    try:
        subprocess.run(command, cwd=OPENWRT_VAGRANT_DIR, check=True, text=True, capture_output=True)
    except subprocess.CalledProcessError as exc:
        raise RuntimeError(
            f"Failed to upload {source} into host {machine} via Vagrant.\n"
            f"STDOUT:\n{exc.stdout}\nSTDERR:\n{exc.stderr}"
        ) from exc


def ensure_guest_scripts_synced() -> None:
    start = time.perf_counter()
    global _SCRIPTS_SYNCED
    global _SCRIPTS_HASH_CACHE
    if not GUEST_SCRIPTS_SOURCE.exists():
        return
    current_hash = _compute_guest_scripts_hash()
    skip_sync = os.environ.get("XP2P_OPENWRT_SKIP_GUEST_SYNC", "").strip().lower() in {"1", "true", "yes"}
    if _SCRIPTS_HASH_CACHE == current_hash and _SCRIPTS_SYNCED:
        elapsed = time.perf_counter() - start
        print(f"TIMING: ensure_guest_scripts_synced: {elapsed:.2f}s")
        return
    cached = _read_cached_scripts_hash()
    if cached == current_hash and cached is not None:
        if skip_sync:
            _SCRIPTS_HASH_CACHE = current_hash
            _SCRIPTS_SYNCED = True
            elapsed = time.perf_counter() - start
            print(f"TIMING: ensure_guest_scripts_synced skipped: {elapsed:.2f}s")
            return
        _SCRIPTS_HASH_CACHE = current_hash
        if _SCRIPTS_SYNCED:
            elapsed = time.perf_counter() - start
            print(f"TIMING: ensure_guest_scripts_synced: {elapsed:.2f}s")
            return
        for machine in OPENWRT_MACHINES:
            _provision_guest_scripts(machine, GUEST_SCRIPTS_ROOT)
        for machine in ALPINE_MACHINES:
            _provision_guest_scripts(machine, ALPINE_GUEST_SCRIPTS_ROOT)
            if ALPINE_DNSMASQ_INSTALLER.exists():
                _provision_file(machine, ALPINE_DNSMASQ_INSTALLER, PurePosixPath("/tmp/dnsmasq-install-alpine.sh"))
        _SCRIPTS_SYNCED = True
        elapsed = time.perf_counter() - start
        print(f"TIMING: ensure_guest_scripts_synced: {elapsed:.2f}s")
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
    _SCRIPTS_SYNCED = True
    elapsed = time.perf_counter() - start
    print(f"TIMING: ensure_guest_scripts_synced: {elapsed:.2f}s")


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


def stop_process(host: Host, pid: int | str) -> None:
    pid_arg = shlex.quote(str(pid))
    script = (
        "pid=\"$1\"; "
        "case \"$pid\" in ''|*[!0-9]*) exit 0;; esac; "
        "kill -0 \"$pid\" >/dev/null 2>&1 || exit 0; "
        "kill \"$pid\" >/dev/null 2>&1 || true; "
        "i=0; "
        "while [ $i -lt 20 ]; do "
        "kill -0 \"$pid\" >/dev/null 2>&1 || exit 0; "
        "sleep 1; "
        "i=$((i+1)); "
        "done; "
        "kill -9 \"$pid\" >/dev/null 2>&1 || true"
    )
    host.run(f"/bin/sh -c {shlex.quote(script)} -- {pid_arg}")
