from __future__ import annotations

import shlex
from typing import Iterable, Optional

from .scripts import server_reverse_script_path
from .utils import run_checked


def _join_command(parts: Iterable[str]) -> str:
    return " ".join(filter(None, parts))


def server_reverse_add(
    host,
    username: str,
    subnets: Iterable[str],
    *,
    server_id: Optional[str] = None,
):
    script = shlex.quote(server_reverse_script_path(host))
    subnet_args = " ".join(
        f"--subnet {shlex.quote(subnet)}" for subnet in subnets
    )
    env_parts = ["XRAY_REVERSE_SUFFIX=.rev"]
    server_arg = ""
    if server_id:
        env_parts.append(f"XRAY_REVERSE_SERVER_ID={shlex.quote(server_id)}")
        server_arg = f"--server {shlex.quote(server_id)}"

    env = " ".join(env_parts)
    command = _join_command(
        [
            env,
            script,
            "add",
            subnet_args,
            server_arg,
            f"--id {shlex.quote(username)}",
        ]
    )
    description = f"add reverse tunnel id={username}"
    if server_id:
        description += f" server={server_id}"
    return run_checked(host, command, description)


def server_reverse_remove(
    host,
    username: str,
    *,
    server_id: Optional[str] = None,
):
    script = shlex.quote(server_reverse_script_path(host))
    env_parts = ["XRAY_REVERSE_SUFFIX=.rev"]
    server_arg = ""
    if server_id:
        env_parts.append(f"XRAY_REVERSE_SERVER_ID={shlex.quote(server_id)}")
        server_arg = f"--server {shlex.quote(server_id)}"

    env = " ".join(env_parts)
    command = _join_command(
        [
            env,
            script,
            "remove",
            server_arg,
            f"--id {shlex.quote(username)}",
        ]
    )
    description = f"remove reverse tunnel id={username}"
    if server_id:
        description += f" server={server_id}"
    return run_checked(host, command, description)


def server_reverse_remove_raw(
    host,
    username: str,
    *,
    server_id: Optional[str] = None,
):
    script = shlex.quote(server_reverse_script_path(host))
    env_parts = ["XRAY_REVERSE_SUFFIX=.rev"]
    server_arg = ""
    if server_id:
        env_parts.append(f"XRAY_REVERSE_SERVER_ID={shlex.quote(server_id)}")
        server_arg = f"--server {shlex.quote(server_id)}"

    env = " ".join(env_parts)
    command = _join_command(
        [
            env,
            script,
            "remove",
            server_arg,
            f"--id {shlex.quote(username)}",
        ]
    )
    return host.run(command)


__all__ = [
    "server_reverse_add",
    "server_reverse_remove",
    "server_reverse_remove_raw",
]
