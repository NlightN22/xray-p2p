from __future__ import annotations

import shlex
from typing import Dict

from .scripts import client_script_path
from .utils import run_checked


def client_script_run(
    host,
    subcommand: str,
    *args: str,
    env: Dict[str, str] | None = None,
    check: bool = True,
    description: str | None = None,
):
    script = shlex.quote(client_script_path(host))
    cmd = f"{script} {shlex.quote(subcommand)}"
    if args:
        arg_str = " ".join(shlex.quote(arg) for arg in args)
        cmd = f"{cmd} {arg_str}"
    if env:
        exports = " ".join(
            f"{key}={shlex.quote(str(value))}"
            for key, value in env.items()
            if value is not None
        )
        if exports:
            cmd = f"{exports} {cmd}"
    if check:
        return run_checked(host, cmd, description or f"client {subcommand}")
    return host.run(cmd)


def client_install(
    host,
    *args: str,
    env: Dict[str, str] | None = None,
    check: bool = True,
    description: str | None = None,
):
    return client_script_run(
        host,
        "install",
        *args,
        env=env,
        check=check,
        description=description or "install client",
    )


def client_remove(host, purge_core: bool = False, check: bool = True):
    remove_args = ["--purge-core"] if purge_core else []
    return client_script_run(
        host,
        "remove",
        *remove_args,
        env=None,
        check=check,
        description="remove client",
    )


def client_is_installed(host) -> bool:
    config_path = shlex.quote("/etc/xray-p2p-client/config.json")
    service_path = shlex.quote("/etc/init.d/xray-p2p-client")
    config_result = host.run(f"test -f {config_path}")
    service_result = host.run(f"test -x {service_path}")
    return config_result.rc == 0 and service_result.rc == 0


__all__ = [
    "client_script_run",
    "client_install",
    "client_remove",
    "client_is_installed",
]
