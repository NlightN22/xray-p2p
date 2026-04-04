from __future__ import annotations

from pathlib import Path
import time

import pytest

from tests.host.win import env as _env
from tests.host.win import tun_full_diagnostics as diag

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

IFACE_TIMEOUT = 120.0
ROUTE_WAIT_INITIAL = 60.0
ROUTE_WAIT_RETRY = 60.0
ROUTE_WAIT_SPLIT = 120.0
ROUTE_POLL_INTERVAL = 2.0
WATCH_RESTART_WINDOW = 35.0


def cleanup_client_install(host, runner) -> None:
    runner("client", "remove", "--all", "--ignore-missing")
    _env.cleanup_xp2p_install(
        host,
        config_dirs=[CLIENT_CONFIG_DIR],
        state_files=CLIENT_STATE_FILES,
    )
def client_tun_name(host) -> str:
    client_cfg = _env.read_toml(host, CLIENT_CONFIG_FILE).get("client") or {}
    tun_name = (client_cfg.get("tun_name") or "").strip()
    return tun_name or "xp2pc"


def default_routes(host) -> list[dict]:
    return _env.get_net_routes(host, "0.0.0.0/0")


def route_id(route: dict) -> tuple[str, int]:
    next_hop = str(route.get("NextHop") or "").strip()
    try:
        index = int(route.get("InterfaceIndex"))
    except (TypeError, ValueError):
        index = -1
    return next_hop, index


def route_metric(route: dict) -> int:
    try:
        return int(route.get("RouteMetric"))
    except (TypeError, ValueError):
        return 1_000_000


def wait_for_tun_adapter(host, tun_name: str) -> tuple[str, int]:
    deadline = time.time() + IFACE_TIMEOUT
    last_error: Exception | None = None
    while time.time() < deadline:
        try:
            return tun_name, _env.get_interface_index(host, tun_name)
        except Exception as exc:
            last_error = exc
        fallback = _find_adapter_by_prefix(host, tun_name)
        if fallback is not None:
            return fallback
        time.sleep(ROUTE_POLL_INTERVAL)
    pytest.fail(f"Interface {tun_name} not available: {last_error}")


def _find_adapter_by_prefix(host, prefix: str) -> tuple[str, int] | None:
    target = _env.ps_quote(prefix)
    script = f"""
$ErrorActionPreference = 'Stop'
$prefix = {target}
try {{
    $adapters = Get-NetAdapter -IncludeHidden -ErrorAction Stop
}} catch {{
    $adapters = Get-NetAdapter -ErrorAction SilentlyContinue
}}
$adapter = $adapters |
    Where-Object {{ $_.Name -like "$prefix*" }} |
    Sort-Object -Property ifIndex |
    Select-Object -First 1
if (-not $adapter) {{
    $adapter = $adapters |
        Where-Object {{ $_.InterfaceDescription -like '*Wintun*' -or $_.InterfaceDescription -like '*Xray Tunnel*' -or $_.Name -like '*Xray Tunnel*' }} |
        Sort-Object -Property ifIndex |
        Select-Object -First 1
}}
if (-not $adapter) {{
    exit 3
}}
Write-Output ("{0}|{1}" -f $adapter.Name, $adapter.ifIndex)
"""
    result = _env.run_powershell(host, script, label="find_net_adapter")
    if result.rc != 0:
        return None
    value = (result.stdout or "").strip().splitlines()
    if not value:
        return None
    parts = value[-1].split("|", 1)
    if len(parts) != 2:
        return None
    try:
        return parts[0].strip(), int(parts[1].strip())
    except ValueError:
        return None


def is_tun_route(route: dict, tun_name: str, tun_index: int) -> bool:
    alias = str(route.get("InterfaceAlias") or "").strip().lower()
    if alias and alias == tun_name.lower():
        return True
    return route_id(route)[1] == tun_index


def wait_for_condition(label: str, predicate) -> None:
    deadline = time.time() + ROUTE_WAIT_INITIAL
    last_debug = ""
    while time.time() < deadline:
        ok, debug = predicate()
        if ok:
            return
        last_debug = debug
        time.sleep(ROUTE_POLL_INTERVAL)
    pytest.fail(f"{label} not reached within {ROUTE_WAIT_INITIAL:.0f}s.\n{last_debug}")


def poll_for_condition(label: str, predicate, timeout: float) -> tuple[bool, str]:
    deadline = time.time() + timeout
    last_debug = ""
    while time.time() < deadline:
        ok, debug = predicate()
        if ok:
            return True, ""
        last_debug = debug
        time.sleep(ROUTE_POLL_INTERVAL)
    return False, f"{label} not reached within {timeout:.0f}s.\n{last_debug}"


def poll_for_full_tunnel(
    host,
    tun_name: str,
    tun_index: int,
    old_default_ids: set[tuple[str, int]],
    endpoint_ips: list[str],
    timeout: float,
    *,
    log_path: Path | None = None,
) -> tuple[bool, str]:
    def _check():
        defaults = default_routes(host)
        tun_routes = [route for route in defaults if is_tun_route(route, tun_name, tun_index)]
        has_tun_default = bool(tun_routes)
        min_metric = min((route_metric(route) for route in defaults), default=None)
        best_is_tun = min_metric is not None and any(
            route_metric(route) == min_metric for route in tun_routes
        )

        missing_bypass: list[str] = []
        for ip in endpoint_ips:
            routes = _env.get_net_routes(host, f"{ip}/32")
            matched = False
            for route in routes:
                if route_id(route) in old_default_ids:
                    matched = True
                    break
            if not matched:
                missing_bypass.append(f"{ip} -> {routes}")

        state_text = ""
        if _env.path_exists(host, _env.CONFIG_ROOT / "xp2p-client.tun-full.json"):
            state_text = _env.read_text(host, _env.CONFIG_ROOT / "xp2p-client.tun-full.json")
        service_log = diag.read_log_tail(host, log_path or CLIENT_SERVICE_LOG)

        ok = has_tun_default and best_is_tun and not missing_bypass
        debug = (
            f"has_tun_default={has_tun_default} best_is_tun={best_is_tun} "
            f"missing_bypass={missing_bypass}\n"
            f"defaults={defaults}\n"
            f"full_state={state_text}\n"
            f"service_log_tail:\n{service_log}"
        )
        return ok, debug

    return poll_for_condition("Full-tunnel routes", _check, timeout)


def poll_for_routes_restored(
    host,
    tun_name: str,
    tun_index: int,
    old_default_ids: set[tuple[str, int]],
    endpoint_ips: list[str],
    timeout: float,
    *,
    log_path: Path | None = None,
) -> tuple[bool, str]:
    def _check():
        defaults = default_routes(host)
        tun_routes = [route for route in defaults if is_tun_route(route, tun_name, tun_index)]
        defaults_restored = any(route_id(route) in old_default_ids for route in defaults)
        bypass_left: list[str] = []
        for ip in endpoint_ips:
            routes = _env.get_net_routes(host, f"{ip}/32")
            if routes:
                bypass_left.append(f"{ip} -> {routes}")

        state_text = ""
        if _env.path_exists(host, _env.CONFIG_ROOT / "xp2p-client.tun-full.json"):
            state_text = _env.read_text(host, _env.CONFIG_ROOT / "xp2p-client.tun-full.json")
        service_log = diag.read_log_tail(host, log_path or CLIENT_SERVICE_LOG)

        ok = not tun_routes and defaults_restored and not bypass_left
        debug = (
            f"tun_routes={tun_routes} defaults_restored={defaults_restored} "
            f"bypass_left={bypass_left}\n"
            f"defaults={defaults}\n"
            f"full_state={state_text}\n"
            f"service_log_tail:\n{service_log}"
        )
        return ok, debug

    return poll_for_condition("Routes restored", _check, timeout)


def expected_tag(host: str) -> str:
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
