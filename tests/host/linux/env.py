from __future__ import annotations

import base64
import json
import shlex
from contextlib import contextmanager
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
DEFAULT_CLIENT = MACHINE_IDS[0]
DEFAULT_SERVER = MACHINE_IDS[1]
DEFAULT_AUX = MACHINE_IDS[2]
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


def _posix(value: str | Path | PurePosixPath) -> str:
    if isinstance(value, (Path, PurePosixPath)):
        return value.as_posix()
    return str(value)


def run_guest_script(host: Host, relative_path: str, *args: str) -> CommandResult:
    script_path = GUEST_SCRIPTS_ROOT / relative_path
    quoted_script = shlex.quote(script_path.as_posix())
    quoted_args = " ".join(shlex.quote(str(arg)) for arg in args)
    command = f"sudo -n /bin/bash {quoted_script}"
    if quoted_args:
        command = f"{command} {quoted_args}"
    return host.run(command)


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

    host.run("sudo -n chmod +x /srv/xray-p2p/scripts/build/build_deb_xp2p.sh >/dev/null 2>&1 || true")

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
    command = f"sudo -n {INSTALL_PATH.as_posix()}"
    if quoted_args:
        command = f"{command} {quoted_args}"
    return host.run(command)


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


def path_exists(host: Host, path: str | Path | PurePosixPath) -> bool:
    result = run_guest_script(host, "scripts/linux/path_exists.sh", _posix(path))
    if result.rc in (0, 3):
        return result.rc == 0
    raise RuntimeError(
        f"Failed to check path {path} (exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
    )


def remove_path(host: Host, path: str | Path | PurePosixPath) -> None:
    result = run_guest_script(host, "scripts/linux/remove_path.sh", _posix(path))
    if result.rc not in (0, 3):
        raise RuntimeError(
            f"Failed to remove path {path} (exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def read_text(host: Host, path: str | Path | PurePosixPath) -> str:
    result = run_guest_script(host, "scripts/linux/read_file.sh", _posix(path))
    if result.rc != 0:
        raise RuntimeError(
            f"Failed to read remote text {path} (exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return result.stdout


def read_json(host: Host, path: str | Path | PurePosixPath) -> dict:
    content = read_text(host, path)
    try:
        return json.loads(content)
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"Failed to parse JSON from {path}: {exc}\nContent:\n{content}") from exc


def write_text(host: Host, path: str | Path | PurePosixPath, content: str) -> None:
    encoded = base64.b64encode(content.encode("utf-8")).decode("ascii")
    result = run_guest_script(
        host,
        "scripts/linux/write_file.sh",
        _posix(path),
        encoded,
    )
    if result.rc != 0:
        raise RuntimeError(
            f"Failed to write remote text {path} (exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def file_sha256(host: Host, path: str | Path | PurePosixPath) -> str:
    result = run_guest_script(host, "scripts/linux/file_sha256.sh", _posix(path))
    if result.rc != 0:
        raise RuntimeError(
            f"Failed to hash remote file {path} (exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return (result.stdout or "").strip()


@contextmanager
def xp2p_run_session(
    host: Host,
    role: str,
    install_dir: str | Path | PurePosixPath,
    config_dir: str,
    log_path: str | Path | PurePosixPath,
):
    install_arg = _posix(install_dir)
    log_arg = _posix(log_path)
    result = run_guest_script(
        host,
        "scripts/linux/start_xp2p_run.sh",
        role,
        install_arg,
        config_dir,
        log_arg,
    )
    if result.rc != 0:
        raise RuntimeError(
            f"Failed to start xp2p {role} run (exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    pid_value = _install_marker("__XP2P_PID__=", result.stdout)
    if not pid_value:
        raise RuntimeError(
            f"xp2p {role} run script did not emit PID marker.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    try:
        yield {"pid": int(pid_value), "log": log_arg}
    finally:
        run_guest_script(host, "scripts/linux/stop_process.sh", pid_value)
