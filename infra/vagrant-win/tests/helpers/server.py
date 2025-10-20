from __future__ import annotations

import shlex
from typing import Dict

import pytest

from .constants import SERVER_CONFIG_DIR, SERVER_SERVICE_PATH, SERVER_SCRIPT_URL
from .scripts import server_user_script_path
from .utils import run_checked


def server_script_run(
    host,
    subcommand: str,
    *args: str,
    env: Dict[str, str] | None = None,
    check: bool = True,
    description: str | None = None,
):
    curl_cmd = f"curl -fsSL {shlex.quote(SERVER_SCRIPT_URL)}"
    tokens = [shlex.quote(subcommand)]
    if args:
        tokens.extend(shlex.quote(arg) for arg in args)
    pipeline = f"{curl_cmd} | sh -s -- {' '.join(tokens)}"
    if env:
        exports = " ".join(
            f"{key}={shlex.quote(str(value))}"
            for key, value in env.items()
            if value is not None
        )
        if exports:
            pipeline = f"{exports} {pipeline}"
    if check:
        return run_checked(host, pipeline, description or f"server {subcommand}")
    return host.run(pipeline)


def server_install(
    host,
    *args: str,
    env: Dict[str, str] | None = None,
    check: bool = True,
    description: str | None = None,
):
    return server_script_run(
        host,
        "install",
        *args,
        env=env,
        check=check,
        description=description or "install server",
    )


def server_remove(host, purge_core: bool = False, check: bool = True):
    remove_args = ["--purge-core"] if purge_core else []
    return server_script_run(
        host,
        "remove",
        *remove_args,
        env=None,
        check=check,
        description="remove server",
    )


def start_port_guard(host, port: int) -> str:
    attempts = [
        "sh -c 'uhttpd -p 0.0.0.0:{port} -f -h /www >/dev/null 2>&1 & echo $!'",
        "sh -c 'busybox httpd -f -p 0.0.0.0:{port} >/dev/null 2>&1 & echo $!'",
        "sh -c 'busybox nc -l -p {port} >/dev/null 2>&1 & echo $!'",
    ]
    for command in attempts:
        result = host.run(command.format(port=port))
        if result.rc != 0:
            continue
        pid = result.stdout.strip()
        if not pid:
            continue
        host.run("sleep 1")
        check = host.run(f"netstat -tln | grep -E '[.:]{port}([^0-9]|$)'")
        if check.rc == 0:
            return pid
        stop_port_guard(host, pid)
    pytest.skip(f"Unable to start port guard for {port}")


def stop_port_guard(host, pid: str | None):
    if not pid:
        return
    host.run(f"kill {pid} >/dev/null 2>&1 || true")
    host.run("sleep 1")


def server_is_installed(host) -> bool:
    inbound_path = shlex.quote(f"{SERVER_CONFIG_DIR}/inbounds.json")
    service_path = shlex.quote(SERVER_SERVICE_PATH)
    config_result = host.run(f"test -f {inbound_path}")
    service_result = host.run(f"test -x {service_path}")
    return config_result.rc == 0 and service_result.rc == 0


def server_user_issue(host, email: str, connection_host: str):
    script = shlex.quote(server_user_script_path(host))
    env = " ".join(
        [
            "XRAY_SHOW_CLIENTS=0",
            "XRAY_AUTO_EMAIL=1",
            "XRAY_SHOW_INTERFACES=0",
            "XRAY_ALLOW_INSECURE=0",
            f"XRAY_SERVER_ADDRESS={shlex.quote(connection_host)}",
        ]
    )
    command = (
        f"{env} {script} issue {shlex.quote(email)} {shlex.quote(connection_host)}"
    )
    return run_checked(host, command, f"issue client {email}")


def server_user_remove(host, email: str):
    script = shlex.quote(server_user_script_path(host))
    env = " ".join(
        [
            "XRAY_SHOW_CLIENTS=0",
            "XRAY_SHOW_INTERFACES=0",
        ]
    )
    command = f"{env} {script} remove {shlex.quote(email)}"
    return run_checked(host, command, f"remove client {email}")


__all__ = [
    "server_script_run",
    "server_install",
    "server_remove",
    "start_port_guard",
    "stop_port_guard",
    "server_is_installed",
    "server_user_issue",
    "server_user_remove",
]
