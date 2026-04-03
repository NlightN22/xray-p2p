from __future__ import annotations

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


def _client_service_log_candidates() -> list[Path]:
    return [
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


def _find_tun_adapter(host, tun_name: str) -> tuple[str, int]:
    target = _env.ps_quote(tun_name)
    script = f"""
$ErrorActionPreference = 'Stop'
$name = {target}
try {{
    $adapters = Get-NetAdapter -IncludeHidden -ErrorAction Stop
}} catch {{
    $adapters = Get-NetAdapter -ErrorAction SilentlyContinue
}}
$adapter = $adapters | Where-Object {{ $_.Name -eq $name }} | Select-Object -First 1
if (-not $adapter) {{
    $adapter = $adapters |
        Where-Object {{ $_.Name -like "$name*" }} |
        Sort-Object -Property ifIndex |
        Select-Object -First 1
}}
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
    result = _env.run_powershell(host, script, label="find_tun_adapter")
    if result.rc != 0:
        pytest.fail(
            "TUN adapter not found.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    value = (result.stdout or "").strip().splitlines()
    if not value:
        pytest.fail("TUN adapter lookup returned empty output.")
    parts = value[-1].split("|", 1)
    if len(parts) != 2:
        pytest.fail(f"Unexpected TUN adapter output: {value[-1]!r}")
    try:
        return parts[0].strip(), int(parts[1].strip())
    except ValueError as exc:
        pytest.fail(f"Unexpected TUN adapter output: {value[-1]!r} ({exc})")


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
    try:
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
        svc.wait_for_service_ready(client_host)

        svc.restart_client_service(xp2p_client_runner)
        try:
            svc.wait_for_service_ready(client_host)
        except pytest.fail.Exception as exc:
            debug = _collect_restart_debug(client_host)
            pytest.fail(
                "Client service failed to reach active after restart.\n"
                f"{debug}\n"
                f"original_error={exc}"
            )

        tun_name = "xp2pc"
        adapter_name, adapter_index = _find_tun_adapter(client_host, tun_name)
        ips = svc.wait_for_tun_ipv4_by_index(client_host, adapter_index, timeout=60.0)
        print(f"INFO: client TUN adapter: name={adapter_name} index={adapter_index} ips={ips}")
        if not ips:
            debug = _collect_restart_debug(client_host)
            pytest.fail(f"TUN adapter has no IPv4 address after restart.\n{debug}")

        log_path = _find_client_service_log(client_host)
        tail = diag.read_log_tail(client_host, log_path)
        print(f"INFO: client service log path: {log_path}")
        print("INFO: client service log tail:")
        print(tail)
        if not (tail or "").strip():
            debug = _collect_restart_debug(client_host)
            pytest.fail(f"Client service log is empty at {log_path}\n{debug}")
    finally:
        _cleanup_client_install(client_host, xp2p_client_runner)
