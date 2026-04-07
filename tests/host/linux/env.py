from __future__ import annotations

import base64
import json
import shlex
import time
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
_DEB_BUILD_READY = False


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


def run_guest_script(
    host: Host,
    relative_path: str,
    *args: str,
    timeout: int | None = None,
) -> CommandResult:
    script_path = GUEST_SCRIPTS_ROOT / relative_path
    quoted_script = shlex.quote(script_path.as_posix())
    quoted_args = " ".join(shlex.quote(str(arg)) for arg in args)
    command = f"sudo -n /bin/bash {quoted_script}"
    if quoted_args:
        command = f"{command} {quoted_args}"
    if timeout is None:
        return host.run(command)
    return host.run(command, timeout=timeout)


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
    host.run(f"sudo -n /bin/sh -c {shlex.quote(script)} -- {pid_arg}")


def run_guest_script_with_env(
    host: Host,
    relative_path: str,
    env: dict[str, str],
    *args: str,
    timeout: int | None = None,
) -> CommandResult:
    script_path = GUEST_SCRIPTS_ROOT / relative_path
    quoted_script = shlex.quote(script_path.as_posix())
    quoted_args = " ".join(shlex.quote(str(arg)) for arg in args)
    env_parts = " ".join(f"{key}={shlex.quote(str(value))}" for key, value in env.items())
    command = f"sudo -n env {env_parts} /bin/bash {quoted_script}"
    if quoted_args:
        command = f"{command} {quoted_args}"
    if timeout is None:
        return host.run(command)
    return host.run(command, timeout=timeout)


def _install_marker(marker: str, output: str | None) -> str | None:
    for line in (output or "").splitlines():
        line = line.strip()
        if line.startswith(marker):
            return line[len(marker) :].strip()
    return None


def ensure_xp2p_installed(machine: str, host: Host) -> dict[str, str]:
    global _DEB_BUILD_READY
    host.run("sudo -n chmod +x /srv/xray-p2p/scripts/build/build_deb_xp2p.sh >/dev/null 2>&1 || true")

    install_timeout = 600
    if _DEB_BUILD_READY:
        timing_label = f"linux install_xp2p {machine} (skip_build)"
        start = time.perf_counter()
        result = run_guest_script_with_env(
            host,
            "scripts/linux/install_xp2p.sh",
            {"XP2P_SKIP_BUILD": "1"},
            timeout=install_timeout,
        )
        print(f"TIMING: {timing_label}: {time.perf_counter() - start:.2f}s")
    else:
        timing_label = f"linux install_xp2p {machine} (build)"
        start = time.perf_counter()
        result = run_guest_script(
            host,
            "scripts/linux/install_xp2p.sh",
            timeout=install_timeout,
        )
        print(f"TIMING: {timing_label}: {time.perf_counter() - start:.2f}s")
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
    _DEB_BUILD_READY = True
    return versions


def ensure_xp2p_installed_cached(machine: str, host: Host) -> dict[str, str]:
    cached = _VERSION_CACHE.get(machine)
    if cached is not None:
        return cached
    return ensure_xp2p_installed(machine, host)


def run_xp2p(host: Host, *args: str) -> CommandResult:
    quoted_args = " ".join(shlex.quote(arg) for arg in args)
    command = f"sudo -n {INSTALL_PATH.as_posix()}"
    if quoted_args:
        command = f"{command} {quoted_args}"
    return host.run(command)


def run_xp2p_with_env(host: Host, env: dict[str, str], *args: str) -> CommandResult:
    quoted_args = " ".join(shlex.quote(arg) for arg in args)
    env_parts = " ".join(f"{key}={shlex.quote(str(value))}" for key, value in env.items())
    command = f"sudo -n env {env_parts} {INSTALL_PATH.as_posix()}"
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
    target = _posix(path)
    quoted = shlex.quote(target)
    result = host.run(
        "sudo -n /bin/sh -c "
        "'if [ -e \"$1\" ]; then exit 0; else exit 3; fi' "
        f"-- {quoted}"
    )
    if result.rc in (0, 3):
        return result.rc == 0
    raise RuntimeError(
        f"Failed to check path {path} (exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
    )


def remove_path(host: Host, path: str | Path | PurePosixPath) -> None:
    target = _posix(path)
    quoted = shlex.quote(target)
    result = host.run(
        "sudo -n /bin/sh -c "
        "'if [ -e \"$1\" ]; then rm -rf \"$1\"; exit 0; fi; exit 3' "
        f"-- {quoted}"
    )
    if result.rc not in (0, 3):
        raise RuntimeError(
            f"Failed to remove path {path} (exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def read_text(host: Host, path: str | Path | PurePosixPath) -> str:
    target = _posix(path)
    quoted = shlex.quote(target)
    result = host.run(
        "sudo -n /bin/sh -c "
        "'if [ ! -f \"$1\" ]; then exit 3; fi; cat \"$1\"' "
        f"-- {quoted}"
    )
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
    path_arg = _posix(path)
    quoted_path = shlex.quote(path_arg)
    quoted_content = shlex.quote(encoded)
    result = host.run(
        "sudo -n /bin/sh -c "
        "'mkdir -p \"$(dirname \"$1\")\"; printf %s \"$2\" | base64 -d >\"$1\"' "
        f"-- {quoted_path} {quoted_content}"
    )
    if result.rc != 0:
        raise RuntimeError(
            f"Failed to write remote text {path} (exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def file_sha256(host: Host, path: str | Path | PurePosixPath) -> str:
    target = _posix(path)
    quoted = shlex.quote(target)
    result = host.run(
        "sudo -n /bin/sh -c "
        "'if [ ! -f \"$1\" ]; then exit 3; fi; sha256sum \"$1\"' "
        f"-- {quoted}"
    )
    if result.rc != 0:
        raise RuntimeError(
            f"Failed to hash remote file {path} (exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return (result.stdout or "").split()[0]


def kill_xp2p_processes(host: Host) -> None:
    host.run(
        "sudo -n /bin/sh -c "
        "'for pattern in "
        "\"xp2p client run\" "
        "\"xp2p server run\" "
        "\"xp2p client deploy\" "
        "\"xp2p server deploy\" "
        "\"/etc/xp2p/bin/xray\" "
        "\"/usr/bin/xp2p\"; do "
        "pkill -f \"$pattern\" >/dev/null 2>&1 || true; "
        "done'"
    )


@contextmanager
def xp2p_run_session(
    host: Host,
    role: str,
    install_dir: str | Path | PurePosixPath,
    config_dir: str,
):
    install_arg = _posix(install_dir)
    result = run_guest_script(
        host,
        "scripts/linux/start_xp2p_run.sh",
        role,
        install_arg,
        config_dir,
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
        yield {"pid": int(pid_value)}
    finally:
        stop_process(host, pid_value)


@contextmanager
def xp2p_run_session_with_env(
    host: Host,
    role: str,
    install_dir: str | Path | PurePosixPath,
    config_dir: str,
    *,
    allow_mismatch: bool = False,
    auto_install: bool = True,
):
    install_arg = _posix(install_dir)
    allow_arg = "1" if allow_mismatch else "0"
    auto_arg = "1" if auto_install else "0"
    result = run_guest_script(
        host,
        "scripts/linux/start_xp2p_run_with_env.sh",
        role,
        install_arg,
        config_dir,
        allow_arg,
        auto_arg,
    )
    if result.rc != 0:
        raise RuntimeError(
            f"Failed to start xp2p {role} run with env (exit {result.rc}).\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    pid_value = _install_marker("__XP2P_PID__=", result.stdout)
    if not pid_value:
        raise RuntimeError(
            f"xp2p {role} run script did not emit PID marker.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    try:
        yield {"pid": int(pid_value)}
    finally:
        stop_process(host, pid_value)
