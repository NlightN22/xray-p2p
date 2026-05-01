from __future__ import annotations

import shlex

from testinfra.backend.base import CommandResult
from testinfra.host import Host

from ._xp2p_paths import INSTALL_PATH


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

