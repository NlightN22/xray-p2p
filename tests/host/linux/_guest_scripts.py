from __future__ import annotations

import shlex

from testinfra.backend.base import CommandResult
from testinfra.host import Host

from ._xp2p_paths import GUEST_SCRIPTS_ROOT


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

