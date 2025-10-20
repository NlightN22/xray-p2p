from __future__ import annotations

import shlex
from typing import Iterable

from .scripts import server_reverse_script_path
from .utils import run_checked


def server_reverse_add(host, username: str, subnets: Iterable[str]):
    script = shlex.quote(server_reverse_script_path(host))
    subnet_args = " ".join(
        f"--subnet {shlex.quote(subnet)}" for subnet in subnets
    )
    env = "XRAY_REVERSE_SUFFIX=.rev"
    command = f"{env} {script} add {subnet_args} {shlex.quote(username)}".strip()
    return run_checked(host, command, f"add reverse tunnel {username}")


def server_reverse_remove(host, username: str):
    script = shlex.quote(server_reverse_script_path(host))
    env = "XRAY_REVERSE_SUFFIX=.rev"
    command = f"{env} {script} remove {shlex.quote(username)}"
    return run_checked(host, command, f"remove reverse tunnel {username}")


def server_reverse_remove_raw(host, username: str):
    script = shlex.quote(server_reverse_script_path(host))
    env = "XRAY_REVERSE_SUFFIX=.rev"
    command = f"{env} {script} remove {shlex.quote(username)}"
    return host.run(command)


__all__ = [
    "server_reverse_add",
    "server_reverse_remove",
    "server_reverse_remove_raw",
]
