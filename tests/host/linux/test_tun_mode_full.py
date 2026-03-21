from __future__ import annotations

from pathlib import PurePosixPath
import time

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import env as linux_env

pytestmark = [pytest.mark.host, pytest.mark.linux]

CLIENT_USER = "full-tun@example.com"
CLIENT_PASSWORD = "full-tun-pass"
ENDPOINT_IP = "198.51.100.10"
ENDPOINT_DOMAIN = "tun-full.example"
ENDPOINT_DOMAIN_IP = "198.51.100.20"
DNS_SERVERS = ["1.1.1.1", "9.9.9.9"]

FULL_STATE_FILE = helpers.CONFIG_ROOT / "xp2p-client.tun-full.json"
RESOLV_CONF = PurePosixPath("/etc/resolv.conf")
SERVICE_TIMEOUT = 90.0
POLL_INTERVAL = 2.0


def _wait_for_service_state(runner, role: str, expected_active: bool) -> None:
    deadline = time.time() + SERVICE_TIMEOUT
    last_result = None
    while time.time() < deadline:
        result = runner(role, "service", "status")
        active = result.rc == 0
        if active == expected_active:
            return
        last_result = result
        time.sleep(POLL_INTERVAL)
    state = "active" if expected_active else "inactive"
    stdout = (last_result.stdout or "") if last_result else ""
    stderr = (last_result.stderr or "") if last_result else ""
    raise AssertionError(
        f"xp2p {role} service did not reach {state} state. "
        f"Last rc: {getattr(last_result, 'rc', 'n/a')}\nSTDOUT:\n{stdout}\nSTDERR:\n{stderr}"
    )


def _start_service(runner, host) -> None:
    host.run("sudo -n systemctl daemon-reload >/dev/null 2>&1 || true")
    runner("client", "service", "stop")
    runner("client", "service", "start", check=True)
    _wait_for_service_state(runner, "client", expected_active=True)


def _stop_service(runner) -> None:
    runner("client", "service", "stop")
    _wait_for_service_state(runner, "client", expected_active=False)


def _list_routes(host, family_flag: str, *args: str) -> list[str]:
    cmd = f"sudo -n ip {family_flag} route show {' '.join(args)}".strip()
    result = host.run(cmd)
    if result.rc != 0:
        raise AssertionError(
            f"Command failed: {cmd}\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return [line.strip() for line in (result.stdout or "").splitlines() if line.strip()]


def _list_default_routes(host, family_flag: str) -> list[str]:
    return _list_routes(host, family_flag, "default")


def _resolve_ipv4(host, name: str) -> str:
    result = host.run(f"getent ahostsv4 {name} | awk 'NR==1{{print $1}}'")
    ip = (result.stdout or "").strip()
    if result.rc != 0 or not ip:
        raise AssertionError(
            f"Failed to resolve {name}.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return ip


def _has_default_via_tun(routes: list[str], tun_name: str) -> bool:
    return any(route.startswith("default ") and f"dev {tun_name}" in route for route in routes)


def _build_bypass_routes(defaults: list[str], ips: list[str], prefix_len: int) -> list[str]:
    routes: list[str] = []
    for ip in ips:
        dest = f"{ip}/{prefix_len}"
        for default in defaults:
            fields = default.split()
            if not fields or fields[0] != "default":
                continue
            fields[0] = dest
            routes.append(" ".join(fields))
    return routes


def _wait_for_condition(label: str, predicate, *, timeout: float = SERVICE_TIMEOUT) -> None:
    deadline = time.time() + timeout
    last_debug = ""
    while time.time() < deadline:
        ok, debug = predicate()
        if ok:
            return
        last_debug = debug
        time.sleep(POLL_INTERVAL)
    raise AssertionError(f"{label} not reached within {timeout:.0f}s.\n{last_debug}")


def _update_client_config(host, **updates) -> None:
    original = helpers.read_text(host, helpers.CLIENT_CONFIG_FILE)
    updated = _update_toml_section(original, "client", updates)
    if updated != original:
        helpers.write_text(host, helpers.CLIENT_CONFIG_FILE, updated)


def _update_toml_section(text: str, section: str, updates: dict) -> str:
    lines = text.splitlines()
    out: list[str] = []
    in_section = False
    section_found = False
    seen_keys: set[str] = set()
    section_header = f"[{section}]"

    for line in lines:
        stripped = line.strip()
        if stripped.startswith("[") and stripped.endswith("]") and not stripped.startswith("[["):
            if in_section:
                for key, value in updates.items():
                    if key not in seen_keys:
                        out.append(f"{key} = {_toml_value(value)}")
                        seen_keys.add(key)
                in_section = False
            section_found = section_found or stripped == section_header
            in_section = stripped == section_header
            out.append(line)
            continue
        if in_section and "=" in stripped:
            key = stripped.split("=", 1)[0].strip()
            if key in updates:
                if key not in seen_keys:
                    out.append(f"{key} = {_toml_value(updates[key])}")
                    seen_keys.add(key)
                continue
        out.append(line)

    if in_section:
        for key, value in updates.items():
            if key not in seen_keys:
                out.append(f"{key} = {_toml_value(value)}")
                seen_keys.add(key)

    if not section_found:
        if out and out[-1].strip():
            out.append("")
        out.append(section_header)
        for key, value in updates.items():
            out.append(f"{key} = {_toml_value(value)}")

    return "\n".join(out) + "\n"


def _toml_value(value) -> str:
    if isinstance(value, list):
        entries = ", ".join(f"\"{entry}\"" for entry in value)
        return f"[{entries}]"
    if isinstance(value, str):
        return f"\"{value}\""
    if isinstance(value, bool):
        return "true" if value else "false"
    if isinstance(value, int):
        return str(value)
    raise TypeError(f"Unsupported TOML value: {value!r}")


def _wait_for_full_tunnel(
    host,
    tun_name: str,
    defaults4: list[str],
    defaults6: list[str],
    bypass_routes: list[str],
    dns_servers: list[str],
    original_resolv: str,
) -> None:
    def _check():
        current_defaults4 = _list_default_routes(host, "-4")
        current_defaults6 = _list_default_routes(host, "-6")
        state_exists = helpers.path_exists(host, FULL_STATE_FILE)
        resolv = helpers.read_text(host, RESOLV_CONF)
        bypass_ok = True
        missing_bypass: list[str] = []
        for route in bypass_routes:
            dest = route.split()[0]
            route_lines = _list_routes(host, "-4", dest)
            if route not in route_lines:
                bypass_ok = False
                missing_bypass.append(f"{dest} -> {route_lines}")

        dns_ok = True
        if dns_servers:
            dns_ok = resolv.startswith("# generated by xp2p full-tunnel") and all(
                f"nameserver {server}" in resolv for server in dns_servers
            )
        defaults_ok = True
        if defaults4 and not _has_default_via_tun(current_defaults4, tun_name):
            defaults_ok = False
        if defaults6 and not _has_default_via_tun(current_defaults6, tun_name):
            defaults_ok = False
        defaults_removed = all(route not in current_defaults4 for route in defaults4) and all(
            route not in current_defaults6 for route in defaults6
        )

        ok = state_exists and defaults_ok and defaults_removed and bypass_ok and dns_ok and resolv != original_resolv
        debug = (
            f"state_exists={state_exists} defaults_ok={defaults_ok} defaults_removed={defaults_removed} "
            f"bypass_ok={bypass_ok} dns_ok={dns_ok}\n"
            f"defaults4={current_defaults4}\n"
            f"defaults6={current_defaults6}\n"
            f"missing_bypass={missing_bypass}\n"
            f"resolv_conf:\n{resolv}"
        )
        return ok, debug

    _wait_for_condition("Full-tunnel state", _check)


def _wait_for_rollback(
    host,
    tun_name: str,
    defaults4: list[str],
    defaults6: list[str],
    bypass_routes: list[str],
    original_resolv: str,
) -> None:
    def _check():
        current_defaults4 = _list_default_routes(host, "-4")
        current_defaults6 = _list_default_routes(host, "-6")
        state_exists = helpers.path_exists(host, FULL_STATE_FILE)
        resolv = helpers.read_text(host, RESOLV_CONF)
        bypass_left: list[str] = []
        for route in bypass_routes:
            dest = route.split()[0]
            route_lines = _list_routes(host, "-4", dest)
            if route_lines:
                bypass_left.append(f"{dest} -> {route_lines}")
        defaults_restored = all(route in current_defaults4 for route in defaults4) and all(
            route in current_defaults6 for route in defaults6
        )
        defaults_ok = (not defaults4 or not _has_default_via_tun(current_defaults4, tun_name)) and (
            not defaults6 or not _has_default_via_tun(current_defaults6, tun_name)
        )
        ok = (not state_exists) and defaults_ok and defaults_restored and not bypass_left and resolv == original_resolv
        debug = (
            f"state_exists={state_exists} defaults_ok={defaults_ok} defaults_restored={defaults_restored} "
            f"bypass_left={bypass_left}\n"
            f"defaults4={current_defaults4}\n"
            f"defaults6={current_defaults6}\n"
            f"resolv_conf:\n{resolv}"
        )
        return ok, debug

    _wait_for_condition("Full-tunnel rollback", _check)


def _wait_for_proxy(
    host,
    tun_name: str,
    defaults4: list[str],
    defaults6: list[str],
) -> None:
    def _check():
        current_defaults4 = _list_default_routes(host, "-4")
        current_defaults6 = _list_default_routes(host, "-6")
        defaults_restored = all(route in current_defaults4 for route in defaults4) and all(
            route in current_defaults6 for route in defaults6
        )
        defaults_ok = (not defaults4 or not _has_default_via_tun(current_defaults4, tun_name)) and (
            not defaults6 or not _has_default_via_tun(current_defaults6, tun_name)
        )
        state_exists = helpers.path_exists(host, FULL_STATE_FILE)
        ok = defaults_ok and defaults_restored and not state_exists
        debug = (
            f"defaults_ok={defaults_ok} defaults_restored={defaults_restored} state_exists={state_exists}\n"
            f"defaults4={current_defaults4}\n"
            f"defaults6={current_defaults6}"
        )
        return ok, debug

    _wait_for_condition("Proxy mode rollback", _check)


def _install_client_endpoint(runner, host_value: str, user: str, password: str) -> None:
    runner(
        "client",
        "install",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
        "--host",
        host_value,
        "--user",
        user,
        "--password",
        password,
        "--force",
        check=True,
    )


def _client_mode(runner, *args: str) -> None:
    runner(
        "client",
        "mode",
        *args,
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
        check=True,
    )


def _client_redirect(runner, *args: str, check: bool = False):
    return runner(
        "client",
        "redirect",
        *args,
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
        "--quiet",
        check=check,
    )


def _client_tun_name(host) -> str:
    client_cfg = helpers.read_client_config(host)
    tun_name = (client_cfg.get("tun_name") or "").strip()
    return tun_name or "xp2pc"


def test_client_tun_mode_full_tunnel_routes_and_dns(client_host, xp2p_client_runner, xp2p_linux_versions):
    _ = xp2p_linux_versions[linux_env.DEFAULT_CLIENT]
    host_entry_added = False
    service_started = False
    original_config = None
    original_resolv = None
    try:
        linux_env.run_guest_script(
            client_host,
            "scripts/linux/update_hosts_entry.sh",
            "add",
            ENDPOINT_DOMAIN_IP,
            ENDPOINT_DOMAIN,
        )
        host_entry_added = True

        _install_client_endpoint(xp2p_client_runner, ENDPOINT_IP, CLIENT_USER, CLIENT_PASSWORD)
        _install_client_endpoint(xp2p_client_runner, ENDPOINT_DOMAIN, CLIENT_USER, CLIENT_PASSWORD)

        original_config = helpers.read_text(client_host, helpers.CLIENT_CONFIG_FILE)
        original_resolv = helpers.read_text(client_host, RESOLV_CONF)
        _update_client_config(client_host, tun_mode="split", dns_servers=DNS_SERVERS)

        _start_service(xp2p_client_runner, client_host)
        service_started = True

        defaults4 = _list_default_routes(client_host, "-4")
        defaults6 = _list_default_routes(client_host, "-6")
        assert defaults4 or defaults6, "Expected at least one default route before full-tunnel"

        resolved_domain_ip = _resolve_ipv4(client_host, ENDPOINT_DOMAIN)
        endpoint_ips = [ENDPOINT_IP, resolved_domain_ip]
        bypass_routes = _build_bypass_routes(defaults4, endpoint_ips, 32)
        tun_name = _client_tun_name(client_host)

        _client_mode(xp2p_client_runner, "tun", "full")
        _wait_for_full_tunnel(
            client_host,
            tun_name,
            defaults4,
            defaults6,
            bypass_routes,
            DNS_SERVERS,
            original_resolv,
        )

        _client_mode(xp2p_client_runner, "tun", "split")
        _wait_for_rollback(
            client_host,
            tun_name,
            defaults4,
            defaults6,
            bypass_routes,
            original_resolv,
        )
        assert helpers.read_client_config(client_host).get("tun_mode") == "split"

        _update_client_config(client_host, tun_mode="full", dns_servers=DNS_SERVERS)
        _wait_for_full_tunnel(
            client_host,
            tun_name,
            defaults4,
            defaults6,
            bypass_routes,
            DNS_SERVERS,
            original_resolv,
        )

        _client_mode(xp2p_client_runner, "proxy")
        _wait_for_proxy(client_host, tun_name, defaults4, defaults6)

        applied = helpers.read_client_applied_state(client_host)
        assert applied.get("tun_enabled") is False
        assert applied.get("mode") == "proxy"
    finally:
        if service_started:
            _stop_service(xp2p_client_runner)
        if original_config is not None:
            helpers.write_text(client_host, helpers.CLIENT_CONFIG_FILE, original_config)
        if original_resolv is not None:
            helpers.write_text(client_host, RESOLV_CONF, original_resolv)
        if host_entry_added:
            linux_env.run_guest_script(
                client_host,
                "scripts/linux/update_hosts_entry.sh",
                "remove",
                ENDPOINT_DOMAIN,
            )


def test_client_redirect_default_route_rejected(client_host, xp2p_client_runner, xp2p_linux_versions):
    _ = xp2p_linux_versions[linux_env.DEFAULT_CLIENT]
    _install_client_endpoint(xp2p_client_runner, ENDPOINT_IP, CLIENT_USER, CLIENT_PASSWORD)

    ipv4_result = _client_redirect(
        xp2p_client_runner,
        "add",
        "--cidr",
        "0.0.0.0/0",
        "--host",
        ENDPOINT_IP,
        check=False,
    )
    assert ipv4_result.rc != 0
    combined = f"{ipv4_result.stdout}\n{ipv4_result.stderr}".lower()
    assert "reserved for tun-mode full" in combined

    ipv6_result = _client_redirect(
        xp2p_client_runner,
        "add",
        "--cidr",
        "::/0",
        "--host",
        ENDPOINT_IP,
        check=False,
    )
    assert ipv6_result.rc != 0
    combined = f"{ipv6_result.stdout}\n{ipv6_result.stderr}".lower()
    assert "reserved for tun-mode full" in combined
