from __future__ import annotations

import json
import shlex
import time
from typing import Any, Dict, Iterable, Tuple

import pytest


SETUP_URL = "https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/xsetup.sh"
SERVER_SCRIPT_URL = "https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/server.sh"
CLIENT_SCRIPT_URL = "https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/client.sh"
SERVER_USER_URL = "https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/server_user.sh"
SERVER_REVERSE_URL = "https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/server_reverse.sh"
SERVER_CONFIG_DIR = "/etc/xray-p2p"
SERVER_CLIENTS_PATH = f"{SERVER_CONFIG_DIR}/config/clients.json"
SERVER_INBOUNDS_PATH = f"{SERVER_CONFIG_DIR}/inbounds.json"
SERVER_ROUTING_PATH = f"{SERVER_CONFIG_DIR}/routing.json"
SERVER_TUNNELS_PATH = f"{SERVER_CONFIG_DIR}/config/tunnels.json"
_PROVISIONED: Dict[Tuple[str, str], object] = {}
_SERVER_SCRIPT: str | None = None
_CLIENT_SCRIPT: str | None = None
_SERVER_USER_SCRIPT: str | None = None
_SERVER_REVERSE_SCRIPT: str | None = None
SERVER_SERVICE_PATH = "/etc/init.d/xray-p2p"


def run_checked(host, command: str, description: str):
    """Run a shell command via testinfra host and assert a zero return code."""
    result = host.run(command)
    assert result.rc == 0, (
        f"{description} failed with rc={result.rc}\n"
        f"stdout:\n{result.stdout}\n"
        f"stderr:\n{result.stderr}"
    )
    return result


def check_iperf_open(host, label: str, target: str):
    """Verify that iperf3 can reach the target address."""
    command = (
        f"iperf3 -c {target} -t 1 -P 1 >/dev/null 2>&1 && echo open || "
        "{ echo closed; false; }"
    )
    last_result = None
    for attempt in range(1, 4):
        result = host.run(command)
        if result.rc == 0 and "open" in result.stdout.strip():
            return result
        last_result = result
        if attempt < 4:
            time.sleep(2)
    assert last_result is not None
    raise AssertionError(
        f"{label} iperf3 check failed after retries (target {target}).\n"
        f"exit={last_result.rc}\nstdout:\n{last_result.stdout}\n"
        f"stderr:\n{last_result.stderr}"
    )


def ensure_stage_one(router_host, user: str, client_lan: str):
    """
    Execute the stage-one provisioning script exactly once per (user, client LAN).
    Returns the cached CommandResult when the script has already been run.
    """
    key = (user, client_lan)
    if key in _PROVISIONED:
        return _PROVISIONED[key]

    status = router_host.run("x2 status")
    if status.rc == 0 and "tunnel" in status.stdout.lower():
        _PROVISIONED[key] = status
        return status

    command = (
        f"curl -fsSL {SETUP_URL} | XRAY_SKIP_PORT_CHECK=1 sh -s -- 10.0.0.1 {user} 10.0.101.0/24 {client_lan}"
    )
    result = router_host.run(command)
    combined_output = f"{result.stdout}\n{result.stderr}"
    if result.rc != 0:
        lower_output = combined_output.lower()
        if "failed to download" in lower_output or "failed to send request" in lower_output:
            pytest.skip(
                f"xsetup for {user} requires network access to downloads.openwrt.org (skipping test).\n"
                f"stdout:\n{result.stdout}\nstderr:\n{result.stderr}"
            )
        raise AssertionError(
            f"xsetup for {user} failed with rc={result.rc}\n"
            f"stdout:\n{result.stdout}\n"
            f"stderr:\n{result.stderr}"
        )
    assert "All steps completed successfully." in combined_output, (
        "xsetup did not report success.\n"
        f"stdout:\n{result.stdout}\nstderr:\n{result.stderr}"
    )

    _PROVISIONED[key] = result
    return result


def _download_script(host, path: str, url: str):
    result = host.run(f"test -x {shlex.quote(path)}")
    if result.rc != 0:
        download_cmd = (
            f"curl -fsSL {shlex.quote(url)} > {shlex.quote(path)} && "
            f"chmod +x {shlex.quote(path)}"
        )
        run_checked(host, download_cmd, f"download script {url}")


def _server_script_path(host) -> str:
    global _SERVER_SCRIPT
    if _SERVER_SCRIPT:
        return _SERVER_SCRIPT
    script_path = "/tmp/server.sh"
    _download_script(host, script_path, SERVER_SCRIPT_URL)
    _SERVER_SCRIPT = script_path
    return script_path


def _client_script_path(host) -> str:
    global _CLIENT_SCRIPT
    if _CLIENT_SCRIPT:
        return _CLIENT_SCRIPT
    script_path = "/tmp/client.sh"
    _download_script(host, script_path, CLIENT_SCRIPT_URL)
    _CLIENT_SCRIPT = script_path
    return script_path


def server_script_run(
    host,
    subcommand: str,
    *args: str,
    env: Dict[str, str] | None = None,
    check: bool = True,
    description: str | None = None,
):
    script = shlex.quote(_server_script_path(host))
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
        return run_checked(host, cmd, description or f"server {subcommand}")
    return host.run(cmd)


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
    result = server_script_run(
        host,
        "remove",
        *remove_args,
        env=None,
        check=check,
        description="remove server",
    )
    return result


def client_script_run(
    host,
    subcommand: str,
    *args: str,
    env: Dict[str, str] | None = None,
    check: bool = True,
    description: str | None = None,
):
    script = shlex.quote(_client_script_path(host))
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
    result = client_script_run(
        host,
        "remove",
        *remove_args,
        env=None,
        check=check,
        description="remove client",
    )
    return result


def client_is_installed(host) -> bool:
    inbound_path = shlex.quote(f"{SERVER_CONFIG_DIR}/inbounds.json")
    service_path = shlex.quote(SERVER_SERVICE_PATH)
    config_result = host.run(f"test -f {inbound_path}")
    service_result = host.run(f"test -x {service_path}")
    return config_result.rc == 0 and service_result.rc == 0


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


def _read_json_file(host, path: str, default: Any):
    quoted = shlex.quote(path)
    result = host.run(f"cat {quoted}")
    if result.rc != 0:
        return default
    content = result.stdout.strip()
    if not content:
        return default
    try:
        return json.loads(content)
    except json.JSONDecodeError as exc:
        raise AssertionError(
            f"Invalid JSON content at {path}.\nstdout:\n{result.stdout}\nstderr:\n{result.stderr}"
        ) from exc


def load_clients_registry(host) -> list[Dict[str, Any]]:
    data = _read_json_file(host, SERVER_CLIENTS_PATH, default=[])
    if isinstance(data, list):
        return data
    if isinstance(data, dict):
        clients = data.get("clients", [])
        if isinstance(clients, list):
            return clients
    return []


def load_inbounds_config(host) -> Dict[str, Any]:
    data = _read_json_file(host, SERVER_INBOUNDS_PATH, default={})
    if isinstance(data, dict):
        return data
    return {}


def get_inbound_client_emails(inbounds: Dict[str, Any]) -> list[str]:
    emails: list[str] = []
    for inbound in inbounds.get("inbounds", []):
        if not isinstance(inbound, dict):
            continue
        settings = inbound.get("settings", {})
        if not isinstance(settings, dict):
            continue
        for client in settings.get("clients", []):
            if isinstance(client, dict):
                email = client.get("email")
                if isinstance(email, str) and email:
                    emails.append(email)
    return emails


def _server_user_script_path(host) -> str:
    global _SERVER_USER_SCRIPT
    if _SERVER_USER_SCRIPT:
        return _SERVER_USER_SCRIPT
    script_path = "/tmp/server_user.sh"
    _download_script(host, script_path, SERVER_USER_URL)
    _SERVER_USER_SCRIPT = script_path
    return script_path


def server_user_issue(host, email: str, connection_host: str):
    script = shlex.quote(_server_user_script_path(host))
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
    script = shlex.quote(_server_user_script_path(host))
    env = " ".join(
        [
            "XRAY_SHOW_CLIENTS=0",
            "XRAY_SHOW_INTERFACES=0",
        ]
    )
    command = f"{env} {script} remove {shlex.quote(email)}"
    return run_checked(host, command, f"remove client {email}")


def load_routing_config(host) -> Dict[str, Any]:
    data = _read_json_file(host, SERVER_ROUTING_PATH, default={})
    if isinstance(data, dict):
        return data
    return {}


def load_reverse_tunnels(host) -> list[Dict[str, Any]]:
    data = _read_json_file(host, SERVER_TUNNELS_PATH, default=[])
    if isinstance(data, list):
        return data
    return []


def get_routing_rules(routing: Dict[str, Any]) -> list[Dict[str, Any]]:
    rules = routing.get("routing", {}).get("rules", [])
    return [rule for rule in rules if isinstance(rule, dict)]


def get_reverse_portals(routing: Dict[str, Any]) -> list[Dict[str, Any]]:
    portals = routing.get("reverse", {}).get("portals", [])
    return [portal for portal in portals if isinstance(portal, dict)]


def _server_reverse_script_path(host) -> str:
    global _SERVER_REVERSE_SCRIPT
    if _SERVER_REVERSE_SCRIPT:
        return _SERVER_REVERSE_SCRIPT
    script_path = "/tmp/server_reverse.sh"
    _download_script(host, script_path, SERVER_REVERSE_URL)
    _SERVER_REVERSE_SCRIPT = script_path
    return script_path


def server_reverse_add(host, username: str, subnets: Iterable[str]):
    script = shlex.quote(_server_reverse_script_path(host))
    subnet_args = " ".join(
        f"--subnet {shlex.quote(subnet)}" for subnet in subnets
    )
    env = "XRAY_REVERSE_SUFFIX=.rev"
    command = f"{env} {script} add {subnet_args} {shlex.quote(username)}".strip()
    return run_checked(host, command, f"add reverse tunnel {username}")


def server_reverse_remove(host, username: str):
    script = shlex.quote(_server_reverse_script_path(host))
    env = "XRAY_REVERSE_SUFFIX=.rev"
    command = f"{env} {script} remove {shlex.quote(username)}"
    return run_checked(host, command, f"remove reverse tunnel {username}")


def server_reverse_remove_raw(host, username: str):
    script = shlex.quote(_server_reverse_script_path(host))
    env = "XRAY_REVERSE_SUFFIX=.rev"
    command = f"{env} {script} remove {shlex.quote(username)}"
    return host.run(command)


def get_interface_ipv4(host, interface: str) -> str:
    """
    Return the first IPv4 address assigned to a given interface on the host.
    """
    result = run_checked(
        host, f"ip -o -4 addr show dev {interface}", f"discover IPv4 on {interface}"
    )
    lines = [line.strip() for line in result.stdout.splitlines() if line.strip()]
    assert lines, f"No IPv4 address reported for interface {interface}."
    first = lines[0].split()
    assert len(first) >= 4, (
        f"Unexpected address output for {interface}.\n"
        f"stdout:\n{result.stdout}\nstderr:\n{result.stderr}"
    )
    ipv4_cidr = first[3]
    address = ipv4_cidr.split("/", 1)[0]
    assert address, (
        f"Failed to parse IPv4 address from interface {interface} output.\n"
        f"stdout:\n{result.stdout}\nstderr:\n{result.stderr}"
    )
    return address




