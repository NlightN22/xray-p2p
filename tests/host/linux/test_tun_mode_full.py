from __future__ import annotations

from pathlib import PurePosixPath
import os
import shlex
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
SECOND_ENDPOINT_IP = "198.51.100.30"
UNRESOLVED_DOMAIN = "tun-full-bad.example.invalid"
REDIRECT_CIDR = "203.0.113.10/32"
DNS_SERVERS = ["1.1.1.1", "9.9.9.9"]

FULL_STATE_FILE = helpers.CONFIG_ROOT / "xp2p-client.tun-full.json"
RESOLV_CONF = PurePosixPath("/etc/resolv.conf")
SERVICE_LOG = helpers.LOG_ROOT / "client" / "service.log"
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


def _default_gateway_and_dev(route: str) -> tuple[str | None, str | None]:
    fields = route.split()
    gateway = dev = None
    if "via" in fields:
        idx = fields.index("via")
        if idx + 1 < len(fields):
            gateway = fields[idx + 1]
    if "dev" in fields:
        idx = fields.index("dev")
        if idx + 1 < len(fields):
            dev = fields[idx + 1]
    return gateway, dev


def _route_matches(route: str, gateway: str | None, dev: str | None) -> bool:
    if gateway and f"via {gateway}" not in route:
        return False
    if dev and f"dev {dev}" not in route:
        return False
    return True


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
        helpers.write_apply_request(host, "client")


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


def _replace_endpoint_address(text: str, hostname: str, address: str) -> str:
    target = f'address = "{hostname}"'
    replacement = f'address = "{address}"'
    if target not in text:
        return text
    return text.replace(target, replacement, 1)


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
        defaults_gateways = [_default_gateway_and_dev(route) for route in defaults4]
        for route in bypass_routes:
            dest = route.split()[0]
            route_lines = _list_routes(host, "-4", dest)
            matched = False
            for gateway, dev in defaults_gateways:
                if any(_route_matches(line, gateway, dev) for line in route_lines):
                    matched = True
                    break
            if not matched:
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
        state_enabled = False
        if state_exists:
            try:
                state = helpers.read_json(host, FULL_STATE_FILE)
                state_enabled = bool(state.get("enabled"))
            except Exception:
                state_enabled = True
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
        ok = (not state_exists or not state_enabled) and defaults_ok and defaults_restored and not bypass_left and resolv == original_resolv
        debug = (
            f"state_exists={state_exists} state_enabled={state_enabled} defaults_ok={defaults_ok} defaults_restored={defaults_restored} "
            f"bypass_left={bypass_left}\n"
            f"defaults4={current_defaults4}\n"
            f"defaults6={current_defaults6}\n"
            f"resolv_conf:\n{resolv}"
        )
        return ok, debug

    _wait_for_condition("Full-tunnel rollback", _check)


def _wait_for_routes_restored(
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
        ok = defaults_ok and defaults_restored and not bypass_left and resolv == original_resolv
        debug = (
            f"defaults_ok={defaults_ok} defaults_restored={defaults_restored} "
            f"bypass_left={bypass_left}\n"
            f"defaults4={current_defaults4}\n"
            f"defaults6={current_defaults6}\n"
            f"resolv_conf:\n{resolv}"
        )
        return ok, debug

    _wait_for_condition("Routes restored", _check)


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
        state_enabled = False
        if state_exists:
            try:
                state = helpers.read_json(host, FULL_STATE_FILE)
                state_enabled = bool(state.get("enabled"))
            except Exception:
                state_enabled = True
        ok = defaults_ok and defaults_restored and (not state_exists or not state_enabled)
        debug = (
            f"defaults_ok={defaults_ok} defaults_restored={defaults_restored} "
            f"state_exists={state_exists} state_enabled={state_enabled}\n"
            f"defaults4={current_defaults4}\n"
            f"defaults6={current_defaults6}"
        )
        return ok, debug

    _wait_for_condition("Proxy mode rollback", _check)


def _wait_for_log_entries(host, path: PurePosixPath, entries: list[str], base: str = "", timeout: float = 60.0) -> None:
    deadline = time.time() + timeout
    last_content = ""
    lowered_entries = [entry.lower() for entry in entries]
    while time.time() < deadline:
        if helpers.path_exists(host, path):
            content = helpers.read_text(host, path)
            last_content = content
            delta = content[len(base) :] if base else content
            lowered = delta.lower()
            if all(entry in lowered for entry in lowered_entries):
                return
        time.sleep(POLL_INTERVAL)
    raise AssertionError(
        f"Log {path} did not contain expected entries.\n"
        f"Missing: {lowered_entries}\n"
        f"Last content:\n{last_content}"
    )


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


def _client_mode(runner, *args: str, check: bool = True):
    return runner(
        "client",
        "mode",
        *args,
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
        check=check,
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


def _routing_rules(host) -> list[dict]:
    data = helpers.read_json(host, helpers.CLIENT_CONFIG_DIR / "routing.json")
    routing = data.get("routing") or {}
    rules = routing.get("rules") or []
    if not isinstance(rules, list):
        raise AssertionError(f"routing.rules should be list, got {type(rules)}")
    return [rule for rule in rules if isinstance(rule, dict)]


def _assert_full_tunnel_rule_last(host, outbound_tag: str) -> None:
    rules = _routing_rules(host)
    assert rules, "routing.json rules list is empty"
    last_rule = rules[-1]
    assert last_rule.get("outboundTag") == outbound_tag, f"Unexpected full-tunnel tag: {last_rule}"
    assert last_rule.get("ip") == ["0.0.0.0/0", "::/0"], f"Unexpected full-tunnel rule: {last_rule}"


def _find_rule_index(rules: list[dict], predicate) -> int:
    for idx, rule in enumerate(rules):
        if predicate(rule):
            return idx
    return -1


def _client_list_entries(runner) -> list[dict[str, str]]:
    result = runner(
        "client",
        "list",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
        check=True,
    )
    lines = [line.strip() for line in (result.stdout or "").splitlines() if line.strip()]
    lines = [
        line for line in lines
        if "HOSTNAME" not in line and "INFO xp2p:" not in line
    ]
    if not lines or lines[0].lower().startswith("no client endpoints"):
        return []
    entries: list[dict[str, str]] = []
    for line in lines:
        parts = line.split()
        if len(parts) >= 2:
            entries.append({"host": parts[0], "tag": parts[1]})
    return entries


def _run_client_mode_interactive(host, selection: str) -> None:
    args = [
        "xp2p",
        "client",
        "mode",
        "tun",
        "full",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
    ]
    quoted = " ".join(shlex.quote(arg) for arg in args)
    command = f"printf {shlex.quote(selection + chr(10))} | sudo -n {quoted}"
    result = host.run(command)
    if result.rc != 0:
        raise AssertionError(
            f"Interactive mode command failed (rc={result.rc}).\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def _assert_verbose_flag_available(host, command: str) -> None:
    result = host.run(f"sudo -n {command} --help")
    combined = f"{result.stdout}\n{result.stderr}"
    assert result.rc == 0, f"{command} --help failed: {combined}"
    assert "--verbose" in combined, f"{command} --help missing --verbose flag"


def test_client_tun_mode_full_tunnel_routes_and_dns(client_host, xp2p_client_runner):
    host_entry_added = False
    redirect_added = False
    service_started = False
    original_config = None
    original_resolv = None
    expected_tag = None
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
        expected_tag = helpers.expected_proxy_tag(ENDPOINT_IP)
        resolved_domain_ip = _resolve_ipv4(client_host, ENDPOINT_DOMAIN)
        updated_config = _replace_endpoint_address(original_config, ENDPOINT_DOMAIN, resolved_domain_ip)
        if updated_config != original_config:
            helpers.write_text(client_host, helpers.CLIENT_CONFIG_FILE, updated_config)
            helpers.write_apply_request(client_host, "client")
            helpers.write_apply_request(client_host, "client")
        _update_client_config(client_host, tun_mode="split", dns_servers=DNS_SERVERS)
        redirect_result = _client_redirect(
            xp2p_client_runner,
            "add",
            "--cidr",
            REDIRECT_CIDR,
            "--host",
            ENDPOINT_IP,
            check=False,
        )
        assert redirect_result.rc == 0, (
            "xp2p client redirect add failed.\n"
            f"STDOUT:\n{redirect_result.stdout}\nSTDERR:\n{redirect_result.stderr}"
        )
        redirect_added = True

        _start_service(xp2p_client_runner, client_host)
        service_started = True

        defaults4 = _list_default_routes(client_host, "-4")
        defaults6 = _list_default_routes(client_host, "-6")
        assert defaults4 or defaults6, "Expected at least one default route before full-tunnel"

        endpoint_ips = [ENDPOINT_IP, resolved_domain_ip]
        bypass_routes = _build_bypass_routes(defaults4, endpoint_ips, 32)
        tun_name = _client_tun_name(client_host)

        log_base = helpers.read_text(client_host, SERVICE_LOG) if helpers.path_exists(client_host, SERVICE_LOG) else ""
        result = _client_mode(xp2p_client_runner, "tun", "full", "--tag", expected_tag, "-V", check=False)
        if result.rc != 0:
            pytest.fail(
                "xp2p client mode tun full failed.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        _wait_for_full_tunnel(
            client_host,
            tun_name,
            defaults4,
            defaults6,
            bypass_routes,
            DNS_SERVERS,
            original_resolv,
        )
        _wait_for_log_entries(
            client_host,
            SERVICE_LOG,
            [
                "full-tunnel default routes captured",
                "full-tunnel bypass routes prepared",
                "full-tunnel default routes set to tun",
                "full-tunnel dns override applied",
                "before",
                "after",
            ],
            base=log_base,
        )
        _assert_full_tunnel_rule_last(client_host, expected_tag)
        rules = _routing_rules(client_host)
        domain_bypass_index = _find_rule_index(
            rules,
            lambda rule: resolved_domain_ip in (rule.get("ip") or []) and rule.get("outboundTag") == "direct",
        )
        assert domain_bypass_index != -1, "Domain endpoint bypass rule should use resolved IP in routing.json"
        redirect_rule_index = _find_rule_index(
            rules,
            lambda rule: REDIRECT_CIDR in (rule.get("ip") or []) and rule.get("outboundTag") == expected_tag,
        )
        assert redirect_rule_index != -1, "Redirect routing rule missing from routing.json"
        assert redirect_rule_index < len(rules) - 1, "Redirect rule should appear before full-tunnel rule"
        assert helpers.read_client_config(client_host).get("full_tunnel_tag") == expected_tag

        _stop_service(xp2p_client_runner)
        service_started = False
        _wait_for_routes_restored(
            client_host,
            tun_name,
            defaults4,
            defaults6,
            bypass_routes,
            original_resolv,
        )
        _start_service(xp2p_client_runner, client_host)
        service_started = True
        _wait_for_full_tunnel(
            client_host,
            tun_name,
            defaults4,
            defaults6,
            bypass_routes,
            DNS_SERVERS,
            original_resolv,
        )

        log_base = helpers.read_text(client_host, SERVICE_LOG) if helpers.path_exists(client_host, SERVICE_LOG) else ""
        result = _client_mode(xp2p_client_runner, "tun", "split", "-V", check=False)
        if result.rc != 0:
            pytest.fail(
                "xp2p client mode tun split failed.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        _wait_for_rollback(
            client_host,
            tun_name,
            defaults4,
            defaults6,
            bypass_routes,
            original_resolv,
        )
        _wait_for_log_entries(
            client_host,
            SERVICE_LOG,
            [
                "full-tunnel default routes removed from tun",
                "full-tunnel bypass routes removed",
                "full-tunnel default routes restored",
                "full-tunnel dns restored",
            ],
            base=log_base,
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

        result = _client_mode(xp2p_client_runner, "tun", "full", "--tag", expected_tag, check=False)
        if result.rc != 0:
            pytest.fail(
                "xp2p client mode tun full failed before remove.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        _wait_for_full_tunnel(
            client_host,
            tun_name,
            defaults4,
            defaults6,
            bypass_routes,
            DNS_SERVERS,
            original_resolv,
        )

        remove_result = xp2p_client_runner(
            "client",
            "remove",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--all",
            "--ignore-missing",
            "--quiet",
            check=False,
        )
        if remove_result.rc != 0:
            pytest.fail(
                "xp2p client remove failed.\n"
                f"STDOUT:\n{remove_result.stdout}\nSTDERR:\n{remove_result.stderr}"
            )
        _wait_for_routes_restored(
            client_host,
            tun_name,
            defaults4,
            defaults6,
            bypass_routes,
            original_resolv,
        )

        _install_client_endpoint(xp2p_client_runner, ENDPOINT_IP, CLIENT_USER, CLIENT_PASSWORD)
        _install_client_endpoint(xp2p_client_runner, ENDPOINT_DOMAIN, CLIENT_USER, CLIENT_PASSWORD)
        _update_client_config(client_host, tun_mode="full", dns_servers=DNS_SERVERS)
        _start_service(xp2p_client_runner, client_host)
        service_started = True
        _wait_for_full_tunnel(
            client_host,
            tun_name,
            defaults4,
            defaults6,
            bypass_routes,
            DNS_SERVERS,
            original_resolv,
        )

    finally:
        if service_started:
            _stop_service(xp2p_client_runner)
        if original_config is not None:
            helpers.write_text(client_host, helpers.CLIENT_CONFIG_FILE, original_config)
            helpers.write_apply_request(client_host, "client")
        if original_resolv is not None:
            helpers.write_text(client_host, RESOLV_CONF, original_resolv)
        if redirect_added and expected_tag:
            _client_redirect(
                xp2p_client_runner,
                "remove",
                "--cidr",
                REDIRECT_CIDR,
                "--tag",
                expected_tag,
                check=False,
            )
        if host_entry_added:
            linux_env.run_guest_script(
                client_host,
                "scripts/linux/update_hosts_entry.sh",
                "remove",
                ENDPOINT_DOMAIN,
            )


def test_client_tun_mode_full_tunnel_routes_restore_after_purge(client_host, xp2p_client_runner):
    if os.environ.get("XP2P_RUN_DESTRUCTIVE_TESTS", "").strip().lower() not in {"1", "true", "yes"}:
        pytest.skip("Set XP2P_RUN_DESTRUCTIVE_TESTS=1 to run destructive package purge test.")

    host_entry_added = False
    service_started = False
    package_removed = False
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
        expected_tag = helpers.expected_proxy_tag(ENDPOINT_IP)
        resolved_domain_ip = _resolve_ipv4(client_host, ENDPOINT_DOMAIN)
        updated_config = _replace_endpoint_address(original_config, ENDPOINT_DOMAIN, resolved_domain_ip)
        if updated_config != original_config:
            helpers.write_text(client_host, helpers.CLIENT_CONFIG_FILE, updated_config)
            helpers.write_apply_request(client_host, "client")
        _update_client_config(client_host, tun_mode="split", dns_servers=DNS_SERVERS)

        _start_service(xp2p_client_runner, client_host)
        service_started = True

        defaults4 = _list_default_routes(client_host, "-4")
        defaults6 = _list_default_routes(client_host, "-6")
        assert defaults4 or defaults6, "Expected at least one default route before full-tunnel"
        tun_name = _client_tun_name(client_host)
        endpoint_ips = [ENDPOINT_IP, resolved_domain_ip]
        bypass_routes = _build_bypass_routes(defaults4, endpoint_ips, 32)

        result = _client_mode(xp2p_client_runner, "tun", "full", "--tag", expected_tag, check=False)
        if result.rc != 0:
            pytest.fail(
                "xp2p client mode tun full failed.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        _wait_for_full_tunnel(
            client_host,
            tun_name,
            defaults4,
            defaults6,
            bypass_routes,
            DNS_SERVERS,
            original_resolv,
        )

        package_remove = client_host.run("sudo -n dpkg -P xp2p")
        if package_remove.rc != 0:
            pytest.fail(
                "dpkg purge failed.\n"
                f"STDOUT:\n{package_remove.stdout}\nSTDERR:\n{package_remove.stderr}"
            )
        package_removed = True
        service_started = False
        _wait_for_routes_restored(
            client_host,
            tun_name,
            defaults4,
            defaults6,
            bypass_routes,
            original_resolv,
        )
    finally:
        if package_removed:
            reinstall = linux_env.run_guest_script(client_host, "scripts/linux/install_xp2p.sh")
            if reinstall.rc != 0:
                pytest.fail(
                    "Failed to reinstall xp2p package.\n"
                    f"STDOUT:\n{reinstall.stdout}\nSTDERR:\n{reinstall.stderr}"
                )
        if service_started and not package_removed:
            _stop_service(xp2p_client_runner)
        if original_config is not None and not package_removed:
            helpers.write_text(client_host, helpers.CLIENT_CONFIG_FILE, original_config)
            helpers.write_apply_request(client_host, "client")
        if original_resolv is not None:
            helpers.write_text(client_host, RESOLV_CONF, original_resolv)
        if original_config is not None and package_removed:
            helpers.write_text(client_host, helpers.CLIENT_CONFIG_FILE, original_config)
            helpers.write_apply_request(client_host, "client")
        if host_entry_added:
            linux_env.run_guest_script(
                client_host,
                "scripts/linux/update_hosts_entry.sh",
                "remove",
                ENDPOINT_DOMAIN,
            )


def test_client_tun_mode_full_tunnel_selection_and_prompt(client_host, xp2p_client_runner):
    linux_env.run_guest_script(
        client_host,
        "scripts/linux/update_hosts_entry.sh",
        "add",
        ENDPOINT_DOMAIN_IP,
        ENDPOINT_DOMAIN,
    )
    try:
        _install_client_endpoint(xp2p_client_runner, ENDPOINT_IP, CLIENT_USER, CLIENT_PASSWORD)
        _install_client_endpoint(xp2p_client_runner, ENDPOINT_DOMAIN, CLIENT_USER, CLIENT_PASSWORD)
        _install_client_endpoint(xp2p_client_runner, SECOND_ENDPOINT_IP, CLIENT_USER, CLIENT_PASSWORD)

        config_hash = helpers.file_sha256(client_host, helpers.CLIENT_CONFIG_FILE)
        routing_hash = helpers.file_sha256(client_host, helpers.CLIENT_CONFIG_DIR / "routing.json")

        quiet_result = _client_mode(xp2p_client_runner, "tun", "full", "--quiet", check=False)
        assert quiet_result.rc != 0
        assert helpers.file_sha256(client_host, helpers.CLIENT_CONFIG_FILE) == config_hash
        assert helpers.file_sha256(client_host, helpers.CLIENT_CONFIG_DIR / "routing.json") == routing_hash
        assert helpers.read_client_config(client_host).get("tun_mode") != "full"

        tag_for_ip = helpers.expected_proxy_tag(ENDPOINT_IP)
        tag_for_domain = helpers.expected_proxy_tag(ENDPOINT_DOMAIN)
        tag_for_second = helpers.expected_proxy_tag(SECOND_ENDPOINT_IP)

        result = _client_mode(xp2p_client_runner, "tun", "full", "--tag", tag_for_ip, check=False)
        assert result.rc == 0, f"Mode with --tag failed: {result.stdout}\n{result.stderr}"
        _assert_full_tunnel_rule_last(client_host, tag_for_ip)
        assert helpers.read_client_config(client_host).get("full_tunnel_tag") == tag_for_ip

        result = _client_mode(xp2p_client_runner, "tun", "split", check=False)
        assert result.rc == 0, f"Mode split failed: {result.stdout}\n{result.stderr}"

        result = _client_mode(xp2p_client_runner, "tun", "full", "--host", ENDPOINT_DOMAIN, check=False)
        assert result.rc == 0, f"Mode with --host failed: {result.stdout}\n{result.stderr}"
        _assert_full_tunnel_rule_last(client_host, tag_for_domain)
        assert helpers.read_client_config(client_host).get("full_tunnel_tag") == tag_for_domain

        result = _client_mode(xp2p_client_runner, "tun", "split", check=False)
        assert result.rc == 0, f"Mode split failed: {result.stdout}\n{result.stderr}"
        _update_client_config(client_host, full_tunnel_tag="")

        entries = _client_list_entries(xp2p_client_runner)
        assert entries, "Expected client list entries for interactive selection"
        selection_index = None
        for idx, entry in enumerate(entries, start=1):
            if entry["tag"] == tag_for_second:
                selection_index = idx
                break
        assert selection_index is not None, "Could not locate interactive selection index"
        _run_client_mode_interactive(client_host, str(selection_index))
        _assert_full_tunnel_rule_last(client_host, tag_for_second)
        assert helpers.read_client_config(client_host).get("full_tunnel_tag") == tag_for_second

        result = _client_mode(xp2p_client_runner, "tun", "split", check=False)
        assert result.rc == 0, f"Mode split failed: {result.stdout}\n{result.stderr}"
    finally:
        linux_env.run_guest_script(
            client_host,
            "scripts/linux/update_hosts_entry.sh",
            "remove",
            ENDPOINT_DOMAIN,
        )


def test_client_verbose_flags_available(client_host):
    _assert_verbose_flag_available(client_host, "xp2p client mode")
    _assert_verbose_flag_available(client_host, "xp2p client run")
    _assert_verbose_flag_available(client_host, "xp2p client service run")


def test_client_redirect_default_route_rejected(client_host, xp2p_client_runner):
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


def test_client_tun_mode_full_unresolved_endpoint_fails(client_host, xp2p_client_runner):
    install_result = xp2p_client_runner(
        "client",
        "install",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
        "--host",
        UNRESOLVED_DOMAIN,
        "--user",
        CLIENT_USER,
        "--password",
        CLIENT_PASSWORD,
        "--force",
        check=False,
    )
    assert install_result.rc != 0, "Expected unresolved endpoint install to fail"
    combined = f"{install_result.stdout}\n{install_result.stderr}".lower()
    assert "resolve endpoint" in combined and UNRESOLVED_DOMAIN in combined
