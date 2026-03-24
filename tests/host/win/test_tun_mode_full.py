from __future__ import annotations

from pathlib import Path
import time

import pytest

from tests.host.win import env as _env

pytestmark = [pytest.mark.host, pytest.mark.win]

CLIENT_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
CLIENT_CONFIG_DIR_NAME = "config-client"
CLIENT_CONFIG_DIR = _env.CONFIG_ROOT / CLIENT_CONFIG_DIR_NAME
CLIENT_CONFIG_FILE = _env.CONFIG_ROOT / "xp2p-client.toml"
CLIENT_STATE_FILES = [
    CLIENT_CONFIG_FILE,
    _env.CONFIG_ROOT / "xp2p-client.state.json",
    _env.CONFIG_ROOT / "xp2p-client.tun-full.json",
]
CLIENT_SERVICE_LOG = _env.LOGS_DIR / "client" / "service.log"

CLIENT_USER = "full-tun@example.com"
CLIENT_PASSWORD = "full-tun-pass"
ENDPOINT_IP = "198.51.100.10"
ENDPOINT_DOMAIN = "tun-full.example"
ENDPOINT_DOMAIN_IP = "198.51.100.20"

SERVICE_TIMEOUT = 90.0
POLL_INTERVAL = 2.0
IFACE_TIMEOUT = 120.0
ROUTE_WAIT_INITIAL = 60.0
ROUTE_WAIT_RETRY = 60.0
ROUTE_WAIT_SPLIT = 60.0
ROUTE_POLL_INTERVAL = 2.0
WATCH_RESTART_WINDOW = 35.0


def _cleanup_client_install(client_host, runner) -> None:
    runner("client", "remove", "--all", "--ignore-missing")
    _env.cleanup_xp2p_install(
        client_host,
        config_dirs=[CLIENT_CONFIG_DIR],
        state_files=CLIENT_STATE_FILES,
    )


def _require_client_service(host) -> None:
    if not _env.service_exists(host, "xp2p-client"):
        pytest.skip("xp2p-client service is not registered; MSI install required.")


def _wait_for_service_state(runner, expected_active: bool) -> None:
    deadline = time.time() + SERVICE_TIMEOUT
    last_stdout = ""
    last_stderr = ""
    while time.time() < deadline:
        result = runner("client", "service", "status")
        last_stdout = result.stdout or ""
        last_stderr = result.stderr or ""
        active = result.rc == 0
        if active == expected_active:
            return
        time.sleep(POLL_INTERVAL)
    state = "active" if expected_active else "inactive"
    pytest.fail(
        f"xp2p client service did not reach {state} state.\n"
        f"STDOUT:\n{last_stdout}\nSTDERR:\n{last_stderr}"
    )


def _install_client_endpoint(runner, host_value: str, user: str, password: str) -> None:
    runner(
        "client",
        "install",
        "--path",
        str(CLIENT_INSTALL_DIR),
        "--config-dir",
        CLIENT_CONFIG_DIR_NAME,
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
        str(CLIENT_INSTALL_DIR),
        "--config-dir",
        CLIENT_CONFIG_DIR_NAME,
        check=check,
    )


def _resolve_ipv4(host, name: str) -> str:
    result = _env.run_guest_script(
        host,
        "scripts/resolve_dns.ps1",
        Name=name,
    )
    if result.rc != 0:
        pytest.fail(
            f"Failed to resolve {name}.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    value = (result.stdout or "").strip().splitlines()
    if not value:
        pytest.fail(f"No address returned for {name}")
    return value[-1].strip()


def _update_client_config(host, **updates) -> None:
    original = _env.read_text(host, CLIENT_CONFIG_FILE)
    updated = _update_toml_section(original, "client", updates)
    if updated != original:
        _env.write_text(host, CLIENT_CONFIG_FILE, updated)


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


def _client_tun_name(host) -> str:
    client_cfg = _env.read_toml(host, CLIENT_CONFIG_FILE).get("client") or {}
    tun_name = (client_cfg.get("tun_name") or "").strip()
    return tun_name or "xp2pc"


def _default_routes(host) -> list[dict]:
    return _env.get_net_routes(host, "0.0.0.0/0")


def _route_id(route: dict) -> tuple[str, int]:
    next_hop = str(route.get("NextHop") or "").strip()
    try:
        index = int(route.get("InterfaceIndex"))
    except (TypeError, ValueError):
        index = -1
    return next_hop, index


def _route_metric(route: dict) -> int:
    try:
        return int(route.get("RouteMetric"))
    except (TypeError, ValueError):
        return 1_000_000


def _wait_for_interface_index(host, name: str) -> int:
    deadline = time.time() + IFACE_TIMEOUT
    last_error: Exception | None = None
    while time.time() < deadline:
        try:
            return _env.get_interface_index(host, name)
        except Exception as exc:
            last_error = exc
        time.sleep(ROUTE_POLL_INTERVAL)
    pytest.fail(f"Interface {name} not available: {last_error}")


def _wait_for_condition(label: str, predicate) -> None:
    deadline = time.time() + ROUTE_WAIT_INITIAL
    last_debug = ""
    while time.time() < deadline:
        ok, debug = predicate()
        if ok:
            return
        last_debug = debug
        time.sleep(ROUTE_POLL_INTERVAL)
    pytest.fail(f"{label} not reached within {ROUTE_WAIT_INITIAL:.0f}s.\n{last_debug}")


def _poll_for_condition(label: str, predicate, timeout: float) -> tuple[bool, str]:
    deadline = time.time() + timeout
    last_debug = ""
    while time.time() < deadline:
        ok, debug = predicate()
        if ok:
            return True, ""
        last_debug = debug
        time.sleep(ROUTE_POLL_INTERVAL)
    return False, f"{label} not reached within {timeout:.0f}s.\n{last_debug}"


def _read_log_tail(host, path: Path, lines: int = 200) -> str:
    if not _env.path_exists(host, path):
        return ""
    target = _env.ps_quote(str(path))
    script = f"""
$ErrorActionPreference = 'Stop'
$content = Get-Content -Path {target} -Tail {int(lines)}
$content
"""
    result = _env.run_powershell(host, script, label="log_tail")
    if result.rc != 0:
        return f"<failed to read {path}>"
    return result.stdout or ""


def _is_tun_route(route: dict, tun_name: str, tun_index: int) -> bool:
    alias = str(route.get("InterfaceAlias") or "").strip().lower()
    if alias and alias == tun_name.lower():
        return True
    return _route_id(route)[1] == tun_index


def _wait_for_full_tunnel(
    host,
    tun_name: str,
    tun_index: int,
    old_default_ids: set[tuple[str, int]],
    endpoint_ips: list[str],
) -> None:
    def _check():
        defaults = _default_routes(host)
        tun_routes = [route for route in defaults if _is_tun_route(route, tun_name, tun_index)]
        has_tun_default = bool(tun_routes)
        min_metric = min((_route_metric(route) for route in defaults), default=None)
        best_is_tun = min_metric is not None and any(
            _route_metric(route) == min_metric for route in tun_routes
        )

        missing_bypass: list[str] = []
        for ip in endpoint_ips:
            routes = _env.get_net_routes(host, f"{ip}/32")
            matched = False
            for route in routes:
                if _route_id(route) in old_default_ids:
                    matched = True
                    break
            if not matched:
                missing_bypass.append(f"{ip} -> {routes}")

        state_text = ""
        if _env.path_exists(host, _env.CONFIG_ROOT / "xp2p-client.tun-full.json"):
            state_text = _env.read_text(host, _env.CONFIG_ROOT / "xp2p-client.tun-full.json")
        service_log = _read_log_tail(host, CLIENT_SERVICE_LOG)

        ok = has_tun_default and best_is_tun and not missing_bypass
        debug = (
            f"has_tun_default={has_tun_default} best_is_tun={best_is_tun} "
            f"missing_bypass={missing_bypass}\n"
            f"defaults={defaults}\n"
            f"full_state={state_text}\n"
            f"service_log_tail:\n{service_log}"
        )
        return ok, debug

    _wait_for_condition("Full-tunnel routes", _check)


def _poll_for_full_tunnel(
    host,
    tun_name: str,
    tun_index: int,
    old_default_ids: set[tuple[str, int]],
    endpoint_ips: list[str],
    timeout: float,
) -> tuple[bool, str]:
    def _check():
        defaults = _default_routes(host)
        tun_routes = [route for route in defaults if _is_tun_route(route, tun_name, tun_index)]
        has_tun_default = bool(tun_routes)
        min_metric = min((_route_metric(route) for route in defaults), default=None)
        best_is_tun = min_metric is not None and any(
            _route_metric(route) == min_metric for route in tun_routes
        )

        missing_bypass: list[str] = []
        for ip in endpoint_ips:
            routes = _env.get_net_routes(host, f"{ip}/32")
            matched = False
            for route in routes:
                if _route_id(route) in old_default_ids:
                    matched = True
                    break
            if not matched:
                missing_bypass.append(f"{ip} -> {routes}")

        state_text = ""
        if _env.path_exists(host, _env.CONFIG_ROOT / "xp2p-client.tun-full.json"):
            state_text = _env.read_text(host, _env.CONFIG_ROOT / "xp2p-client.tun-full.json")
        service_log = _read_log_tail(host, CLIENT_SERVICE_LOG)

        ok = has_tun_default and best_is_tun and not missing_bypass
        debug = (
            f"has_tun_default={has_tun_default} best_is_tun={best_is_tun} "
            f"missing_bypass={missing_bypass}\n"
            f"defaults={defaults}\n"
            f"full_state={state_text}\n"
            f"service_log_tail:\n{service_log}"
        )
        return ok, debug

    return _poll_for_condition("Full-tunnel routes", _check, timeout)


def _wait_for_routes_restored(
    host,
    tun_name: str,
    tun_index: int,
    old_default_ids: set[tuple[str, int]],
    endpoint_ips: list[str],
) -> None:
    def _check():
        defaults = _default_routes(host)
        tun_routes = [route for route in defaults if _is_tun_route(route, tun_name, tun_index)]
        defaults_restored = any(_route_id(route) in old_default_ids for route in defaults)
        bypass_left: list[str] = []
        for ip in endpoint_ips:
            routes = _env.get_net_routes(host, f"{ip}/32")
            if routes:
                bypass_left.append(f"{ip} -> {routes}")

        state_text = ""
        if _env.path_exists(host, _env.CONFIG_ROOT / "xp2p-client.tun-full.json"):
            state_text = _env.read_text(host, _env.CONFIG_ROOT / "xp2p-client.tun-full.json")
        service_log = _read_log_tail(host, CLIENT_SERVICE_LOG)

        ok = not tun_routes and defaults_restored and not bypass_left
        debug = (
            f"tun_routes={tun_routes} defaults_restored={defaults_restored} "
            f"bypass_left={bypass_left}\n"
            f"defaults={defaults}\n"
            f"full_state={state_text}\n"
            f"service_log_tail:\n{service_log}"
        )
        return ok, debug

    _wait_for_condition("Routes restored", _check)


def _poll_for_routes_restored(
    host,
    tun_name: str,
    tun_index: int,
    old_default_ids: set[tuple[str, int]],
    endpoint_ips: list[str],
    timeout: float,
) -> tuple[bool, str]:
    def _check():
        defaults = _default_routes(host)
        tun_routes = [route for route in defaults if _is_tun_route(route, tun_name, tun_index)]
        defaults_restored = any(_route_id(route) in old_default_ids for route in defaults)
        bypass_left: list[str] = []
        for ip in endpoint_ips:
            routes = _env.get_net_routes(host, f"{ip}/32")
            if routes:
                bypass_left.append(f"{ip} -> {routes}")

        state_text = ""
        if _env.path_exists(host, _env.CONFIG_ROOT / "xp2p-client.tun-full.json"):
            state_text = _env.read_text(host, _env.CONFIG_ROOT / "xp2p-client.tun-full.json")
        service_log = _read_log_tail(host, CLIENT_SERVICE_LOG)

        ok = not tun_routes and defaults_restored and not bypass_left
        debug = (
            f"tun_routes={tun_routes} defaults_restored={defaults_restored} "
            f"bypass_left={bypass_left}\n"
            f"defaults={defaults}\n"
            f"full_state={state_text}\n"
            f"service_log_tail:\n{service_log}"
        )
        return ok, debug

    return _poll_for_condition("Routes restored", _check, timeout)


def _expected_tag(host: str) -> str:
    cleaned = host.strip().lower()
    result = []
    last_dash = False
    for char in cleaned:
        if char.isalnum():
            result.append(char)
            last_dash = False
            continue
        if char == "-":
            result.append(char)
            last_dash = False
            continue
        if not last_dash:
            result.append("-")
            last_dash = True
    sanitized = "".join(result).strip("-")
    if not sanitized:
        sanitized = "endpoint"
    return f"proxy-{sanitized}"


def _add_hosts_entry(host, ip: str, hostname: str) -> None:
    result = _env.run_guest_script(
        host,
        "scripts/update_hosts_entry.ps1",
        Action="Add",
        HostName=hostname,
        IPAddress=ip,
    )
    if result.rc != 0:
        pytest.fail(
            "Failed to add hosts entry.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def _remove_hosts_entry(host, hostname: str) -> None:
    _env.run_guest_script(
        host,
        "scripts/update_hosts_entry.ps1",
        Action="Remove",
        HostName=hostname,
    )


def _start_client_service(runner) -> None:
    runner("client", "service", "start", check=True)
    _wait_for_service_state(runner, expected_active=True)


def _stop_client_service(runner) -> None:
    runner("client", "service", "stop", check=True)
    _wait_for_service_state(runner, expected_active=False)


def test_windows_client_tun_mode_full_routes(client_host, xp2p_client_runner):
    _cleanup_client_install(client_host, xp2p_client_runner)
    _require_client_service(client_host)
    host_entry_added = False
    service_started = False
    try:
        _add_hosts_entry(client_host, ENDPOINT_DOMAIN_IP, ENDPOINT_DOMAIN)
        host_entry_added = True

        _install_client_endpoint(xp2p_client_runner, ENDPOINT_IP, CLIENT_USER, CLIENT_PASSWORD)
        _install_client_endpoint(xp2p_client_runner, ENDPOINT_DOMAIN, CLIENT_USER, CLIENT_PASSWORD)

        resolved_domain_ip = _resolve_ipv4(client_host, ENDPOINT_DOMAIN)
        original_config = _env.read_text(client_host, CLIENT_CONFIG_FILE)
        updated_config = _replace_endpoint_address(original_config, ENDPOINT_DOMAIN, resolved_domain_ip)
        if updated_config != original_config:
            _env.write_text(client_host, CLIENT_CONFIG_FILE, updated_config)

        _update_client_config(client_host, tun_mode="split")
        _client_mode(xp2p_client_runner, "tun", "split", check=True)
        _start_client_service(xp2p_client_runner)
        service_started = True

        defaults = _default_routes(client_host)
        assert defaults, "Expected at least one default route before full-tunnel"
        old_default_ids = {_route_id(route) for route in defaults if _route_id(route)[0]}
        assert old_default_ids, "Failed to capture default gateway routes"

        expected_tag = _expected_tag(ENDPOINT_IP)
        result = _client_mode(xp2p_client_runner, "tun", "full", "--tag", expected_tag, check=False)
        if result.rc != 0:
            pytest.fail(
                "xp2p client mode tun full failed.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        client_cfg = _env.read_toml(client_host, CLIENT_CONFIG_FILE).get("client") or {}
        assert client_cfg.get("tun_mode") == "full", "client.tun_mode was not updated to full"
        assert client_cfg.get("full_tunnel_tag") == expected_tag, "client.full_tunnel_tag was not updated"

        tun_name = _client_tun_name(client_host)
        tun_index = _wait_for_interface_index(client_host, tun_name)
        endpoint_ips = [ENDPOINT_IP, resolved_domain_ip]
        ok, debug = _poll_for_full_tunnel(
            client_host,
            tun_name,
            tun_index,
            old_default_ids,
            endpoint_ips,
            ROUTE_WAIT_INITIAL,
        )
        if not ok:
            time.sleep(WATCH_RESTART_WINDOW)
            result = _client_mode(
                xp2p_client_runner,
                "tun",
                "full",
                "--tag",
                expected_tag,
                check=False,
            )
            if result.rc != 0:
                pytest.fail(
                    "xp2p client mode tun full retry failed.\n"
                    f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}\n{debug}"
                )
            ok, debug = _poll_for_full_tunnel(
                client_host,
                tun_name,
                tun_index,
                old_default_ids,
                endpoint_ips,
                ROUTE_WAIT_RETRY,
            )
            if not ok:
                pytest.fail(debug)

        result = _client_mode(xp2p_client_runner, "tun", "split", check=False)
        if result.rc != 0:
            pytest.fail(
                "xp2p client mode tun split failed.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        ok, debug = _poll_for_routes_restored(
            client_host,
            tun_name,
            tun_index,
            old_default_ids,
            endpoint_ips,
            ROUTE_WAIT_SPLIT,
        )
        if not ok:
            pytest.fail(debug)
    finally:
        if service_started:
            _stop_client_service(xp2p_client_runner)
        if host_entry_added:
            _remove_hosts_entry(client_host, ENDPOINT_DOMAIN)
        _cleanup_client_install(client_host, xp2p_client_runner)
