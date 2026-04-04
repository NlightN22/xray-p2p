from __future__ import annotations

import json
from pathlib import Path

import pytest

from tests.host.win import env as _env
from tests.host.win import tun_full_diagnostics as diag
from tests.host.win import tun_full_service as svc

pytestmark = [pytest.mark.host, pytest.mark.win]

CLIENT_CONFIG_DIR_NAME = "config-client"
CLIENT_CONFIG_DIR = _env.CONFIG_ROOT / CLIENT_CONFIG_DIR_NAME
CLIENT_STATE_FILES = [
    _env.CONFIG_ROOT / "xp2p-client.toml",
    _env.CONFIG_ROOT / "xp2p-client.state.json",
    _env.CONFIG_ROOT / "xp2p-client.tun-full.json",
]
SYNCED_LOG_ROOT = Path(r"C:\xp2p\build\logs\tests")
SYNCED_SERVICE_LOG = SYNCED_LOG_ROOT / "client" / "service.log"
SERVICE_REG_PATH = r"HKLM:\SYSTEM\CurrentControlSet\Services\xp2p-client"


def _client_service_log_candidates() -> list[Path]:
    return [
        SYNCED_SERVICE_LOG,
        _env.LOGS_DIR / "client" / "service.log",
        _env.CONFIG_ROOT / "logs" / "client" / "service.log",
        _env.CONFIG_ROOT / _env.APPLY_DIR_NAME / "pending" / "logs" / "client" / "service.log",
    ]


def _find_client_service_log(host) -> Path:
    for path in _client_service_log_candidates():
        if _env.path_exists(host, path):
            return path
    tried = ", ".join(str(path) for path in _client_service_log_candidates())
    pytest.fail(f"Client service log not found. Tried: {tried}")


def _cleanup_client_install(client_host, runner) -> None:
    runner("client", "remove", "--all", "--ignore-missing")
    _env.cleanup_xp2p_install(
        client_host,
        config_dirs=[CLIENT_CONFIG_DIR],
        state_files=CLIENT_STATE_FILES,
    )


def _set_service_log_root(host, log_root: Path) -> list[str]:
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


def _restore_service_env(host, entries: list[str]) -> None:
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


def _collect_restart_debug(host) -> str:
    log_path = None
    for candidate in _client_service_log_candidates():
        if _env.path_exists(host, candidate):
            log_path = candidate
            break
    log_tail = ""
    if log_path is not None:
        log_tail = diag.read_log_tail(host, log_path)
    else:
        log_path = Path("<missing>")

    dump_path = Path(r"C:\Windows\Temp\xp2p-restart-net-state.log")
    result = _env.run_guest_script(
        host,
        "scripts/dump_net_state.ps1",
        OutputPath=str(dump_path),
        Label="restart-debug",
    )
    if result.rc != 0:
        net_state = f"<failed to dump net state>\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
    else:
        net_state = _env.read_text(host, dump_path) if _env.path_exists(host, dump_path) else "<net state missing>"

    return (
        f"log_path={log_path}\n"
        f"log_tail:\n{log_tail}\n"
        f"net_state:\n{net_state}"
    )

def test_client_service_restart_logs(client_host, xp2p_client_runner) -> None:
    if not _env.service_exists(client_host, "xp2p-client"):
        pytest.skip("xp2p-client service is not registered; MSI install required.")

    _cleanup_client_install(client_host, xp2p_client_runner)
    original_env: list[str] = []
    try:
        _env.run_powershell(
            client_host,
            f"New-Item -ItemType Directory -Path {_env.ps_quote(str(SYNCED_LOG_ROOT))} -Force | Out-Null",
            label="ensure_synced_log_root",
        )
        original_env = _set_service_log_root(client_host, SYNCED_LOG_ROOT)
        xp2p_client_runner(
            "client",
            "install",
            "--host",
            "10.62.10.10",
            "--user",
            "restart@example.com",
            "--password",
            "restart-pass",
            "--tun-mode",
            "split",
            "--force",
            check=True,
            )

        svc.start_client_service(xp2p_client_runner)
        svc.wait_for_apply_request_clear(client_host)
        first_outcome = svc.wait_for_service_outcome(client_host, log_path=SYNCED_SERVICE_LOG)
        print(f"INFO: initial service outcome={first_outcome}")

        svc.restart_client_service(xp2p_client_runner)
        try:
            restart_outcome = svc.wait_for_service_outcome(client_host, log_path=SYNCED_SERVICE_LOG)
            print(f"INFO: restart service outcome={restart_outcome}")
        except pytest.fail.Exception as exc:
            debug = _collect_restart_debug(client_host)
            pytest.fail(
                "Client service failed to reach active after restart.\n"
                f"{debug}\n"
                f"original_error={exc}"
            )

        log_path = _find_client_service_log(client_host)
        tail = diag.read_log_tail(client_host, log_path)
        print(f"INFO: client service log path: {log_path}")
        print("INFO: client service log tail:")
        print(tail)
        if not (tail or "").strip():
            debug = _collect_restart_debug(client_host)
            pytest.fail(f"Client service log is empty at {log_path}\n{debug}")
    finally:
        if original_env is not None:
            _restore_service_env(client_host, original_env)
        _env.remove_path(client_host, SYNCED_SERVICE_LOG)
        _cleanup_client_install(client_host, xp2p_client_runner)
