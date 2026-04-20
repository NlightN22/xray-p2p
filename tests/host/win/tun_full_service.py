from __future__ import annotations

import json
import time
from pathlib import Path

import pytest

from tests.host.win import env as _env
from tests.host.win.flows import apply as apply_flow
from tests.host.win import tun_full_diagnostics as diag
from tests.host.win import tun_full_helpers as tun

SERVICE_TIMEOUT = 90.0
POLL_INTERVAL = 2.0
CLIENT_CONFIG = _env.CONFIG_ROOT / "xp2p-client.toml"
CLIENT_SERVICE_LOG = _env.LOGS_DIR / "client" / "service.log"
SERVICE_REG_PATH = r"HKLM:\SYSTEM\CurrentControlSet\Services\xp2p-client"
SUCCESS_MARKERS = {
    "proxy": "socks health check ok",
    "tun": "tun ipv4 available",
}
FAIL_MARKERS = [
    "context cancel",
    "xray-core health check failed",
    "xp2p client service failed",
    "failed to start xp2p",
]


def require_client_service(host) -> None:
    if not _env.service_exists(host, "xp2p-client"):
        pytest.skip("xp2p-client service is not registered; MSI install required.")


def wait_for_service_state(runner, expected_active: bool) -> None:
    wait_for_role_service_state(runner, "client", expected_active)


def start_client_service(runner) -> None:
    start_service(runner, "client")


def stop_client_service(runner) -> None:
    stop_service(runner, "client")


def restart_client_service(runner) -> None:
    restart_service(runner, "client")


def wait_for_role_service_state(runner, role: str, expected_active: bool) -> None:
    deadline = time.time() + SERVICE_TIMEOUT
    last_stdout = ""
    last_stderr = ""
    while time.time() < deadline:
        result = runner(role, "service", "status")
        last_stdout = result.stdout or ""
        last_stderr = result.stderr or ""
        active = result.rc == 0
        if active == expected_active:
            return
        time.sleep(POLL_INTERVAL)
    state = "active" if expected_active else "inactive"
    pytest.fail(
        f"xp2p {role} service did not reach {state} state.\n"
        f"STDOUT:\n{last_stdout}\nSTDERR:\n{last_stderr}"
    )


def wait_for_apply_request_clear(host, timeout: float = 90.0) -> None:
    apply_flow.wait_for_apply_request_clear(
        host,
        timeout=timeout,
        poll_seconds=POLL_INTERVAL,
    )


def wait_for_apply_request_set(host, timeout: float = 60.0) -> None:
    apply_flow.wait_for_apply_request_set(
        host,
        timeout=timeout,
        poll_seconds=POLL_INTERVAL,
    )


def wait_for_client_config(host, timeout: float = 90.0) -> None:
    deadline = time.time() + timeout
    while time.time() < deadline:
        if _env.path_exists(host, CLIENT_CONFIG):
            return
        time.sleep(POLL_INTERVAL)
    pytest.fail(f"xp2p-client.toml did not appear after {timeout} seconds.")


def set_service_log_root(host, log_root: Path) -> list[str]:
    log_root_str = _env.ps_quote(str(log_root))
    script = f"""
$ErrorActionPreference = 'Stop'
$path = '{SERVICE_REG_PATH}'
$existing = @()
try {{
    $value = (Get-ItemProperty -Path $path -Name Environment -ErrorAction SilentlyContinue).Environment
    if ($value) {{
        $existing = @($value)
    }}
}} catch {{}}
$filtered = @()
foreach ($entry in $existing) {{
    if ($entry -and $entry -notlike 'XP2P_LOG_ROOT=*') {{
        $filtered += $entry
    }}
}}
$filtered += ('XP2P_LOG_ROOT=' + {log_root_str})
$envValue = [string[]]$filtered
Set-ItemProperty -Path $path -Name Environment -Value $envValue
$existing | ConvertTo-Json -Compress
"""
    result = _env.run_powershell(host, script, label="set_service_log_root")
    if result.rc != 0:
        pytest.fail(
            "Failed to set service log root.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    payload = (result.stdout or "").strip()
    if not payload:
        return []
    try:
        data = json.loads(payload)
    except json.JSONDecodeError as exc:
        pytest.fail(f"Failed to parse service env snapshot: {payload!r} ({exc})")
    if data is None:
        return []
    if isinstance(data, list):
        return [str(item) for item in data if str(item).strip()]
    return [str(data)]


def restore_service_env(host, entries: list[str]) -> None:
    payload = json.dumps(entries)
    script = f"""
$ErrorActionPreference = 'Stop'
$path = '{SERVICE_REG_PATH}'
$payload = { _env.ps_quote(payload) }
$entries = @()
if ($payload -and $payload -ne 'null') {{
    $entries = $payload | ConvertFrom-Json
}}
if (-not $entries) {{
    Remove-ItemProperty -Path $path -Name Environment -ErrorAction SilentlyContinue
    exit 0
}}
Set-ItemProperty -Path $path -Name Environment -Value $entries
"""
    result = _env.run_powershell(host, script, label="restore_service_env")
    if result.rc != 0:
        pytest.fail(
            "Failed to restore service environment.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def wait_for_service_ready(host, log_path: Path | None = None, timeout: float = 90.0) -> None:
    deadline = time.time() + timeout
    last_tail = ""
    if log_path is None:
        log_path = CLIENT_SERVICE_LOG
    while time.time() < deadline:
        if _env.path_exists(host, log_path):
            last_tail = diag.read_log_tail(host, log_path)
            if "socks health check ok" in (last_tail or "").lower():
                return
        time.sleep(POLL_INTERVAL)
    pytest.fail(
        "xp2p service did not report socks health check ok.\n"
        f"log_path={log_path}\n"
        f"log_tail:\n{last_tail}"
    )


def wait_for_service_outcome(host, log_path: Path | None = None, timeout: float = 120.0) -> str:
    deadline = time.time() + timeout
    if log_path is None:
        log_path = CLIENT_SERVICE_LOG
    last_tail = ""
    seen: set[str] = set()
    if _env.path_exists(host, log_path):
        last_tail = diag.read_log_tail(host, log_path)
        tail_lower = (last_tail or "").lower()
        if SUCCESS_MARKERS["proxy"] in tail_lower:
            return "proxy"
        if SUCCESS_MARKERS["tun"] in tail_lower:
            return "tun"
        if any(marker in tail_lower for marker in FAIL_MARKERS):
            pytest.fail(
                "xp2p service restart failed based on log marker.\n"
                f"log_path={log_path}\n"
                f"log_tail:\n{last_tail}"
            )
        seen = {line.strip() for line in last_tail.splitlines() if line.strip()}
    while time.time() < deadline:
        if _env.path_exists(host, log_path):
            last_tail = diag.read_log_tail(host, log_path)
            tail_lower = (last_tail or "").lower()
            if SUCCESS_MARKERS["proxy"] in tail_lower:
                return "proxy"
            if SUCCESS_MARKERS["tun"] in tail_lower:
                return "tun"
            if any(marker in tail_lower for marker in FAIL_MARKERS):
                pytest.fail(
                    "xp2p service restart failed based on log marker.\n"
                    f"log_path={log_path}\n"
                    f"log_tail:\n{last_tail}"
                )
            for raw in last_tail.splitlines():
                line = raw.strip()
                if not line or line in seen:
                    continue
                lower = line.lower()
                seen.add(line)
        time.sleep(POLL_INTERVAL)
    pytest.fail(
        "xp2p service did not reach a restart outcome within timeout.\n"
        f"log_path={log_path}\n"
        f"log_tail:\n{last_tail}"
    )


def wait_for_service_marker(
    host,
    marker: str,
    *,
    log_path: Path | None = None,
    timeout: float = 120.0,
) -> None:
    deadline = time.time() + timeout
    if log_path is None:
        log_path = CLIENT_SERVICE_LOG
    last_tail = ""
    marker_lower = marker.lower()
    while time.time() < deadline:
        if _env.path_exists(host, log_path):
            last_tail = diag.read_log_tail(host, log_path)
            tail_lower = (last_tail or "").lower()
            if marker_lower in tail_lower:
                return
            if any(marker in tail_lower for marker in FAIL_MARKERS):
                pytest.fail(
                    "xp2p service restart failed based on log marker.\n"
                    f"log_path={log_path}\n"
                    f"log_tail:\n{last_tail}"
                )
        time.sleep(POLL_INTERVAL)
    pytest.fail(
        "xp2p service did not reach expected log marker within timeout.\n"
        f"marker={marker}\n"
        f"log_path={log_path}\n"
        f"log_tail:\n{last_tail}"
    )


def wait_for_tun_ipv4_by_index(host, if_index: int, timeout: float = 60.0) -> list[str]:
    deadline = time.time() + timeout
    last_stdout = ""
    last_stderr = ""
    while time.time() < deadline:
        script = f"""
$ErrorActionPreference = 'Stop'
$index = {int(if_index)}
$ips = Get-NetIPAddress -AddressFamily IPv4 -InterfaceIndex $index -ErrorAction SilentlyContinue |
    Where-Object {{ $_.IPAddress -ne '127.0.0.1' -and $_.IPAddress -notlike '169.254.*' -and $_.IPAddress -ne '0.0.0.0' }} |
    Select-Object -ExpandProperty IPAddress
if (-not $ips) {{
    exit 3
}}
$ips
"""
        result = _env.run_powershell(host, script, label="wait_for_tun_ipv4")
        last_stdout = result.stdout or ""
        last_stderr = result.stderr or ""
        if result.rc == 0:
            return [line.strip() for line in last_stdout.splitlines() if line.strip()]
        time.sleep(POLL_INTERVAL)
    pytest.fail(
        f"TUN IPv4 did not appear for interface index {if_index} after {timeout:.0f}s.\n"
        f"STDOUT:\n{last_stdout}\nSTDERR:\n{last_stderr}"
    )


def wait_for_tun_ipv4(host, tun_name: str, timeout: float = 60.0) -> tuple[str, int, list[str]]:
    adapter_name, adapter_index = tun.wait_for_tun_adapter(host, tun_name)
    ips = wait_for_tun_ipv4_by_index(host, adapter_index, timeout=timeout)
    return adapter_name, adapter_index, ips


def start_service(runner, role: str) -> None:
    runner(role, "service", "start", check=True)
    wait_for_role_service_state(runner, role, expected_active=True)


def stop_service(runner, role: str) -> None:
    runner(role, "service", "stop", check=True)
    wait_for_role_service_state(runner, role, expected_active=False)


def restart_service(runner, role: str) -> None:
    runner(role, "service", "restart", check=True)
    wait_for_role_service_state(runner, role, expected_active=True)
