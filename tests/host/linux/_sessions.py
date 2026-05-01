from __future__ import annotations

from contextlib import contextmanager
from pathlib import Path, PurePosixPath

from testinfra.host import Host

from ._guest_scripts import run_guest_script
from ._process import stop_process
from ._util import _install_marker, _posix


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

