from __future__ import annotations

import base64
import json
import time
from contextlib import contextmanager
from pathlib import Path

import pytest

from tests.host.win import env as win_env
from tests.host.win import tun_full_helpers as tun_helpers
from tests.host.win.flows import apply as apply_flow

INSTALL_ROOT = Path(r"C:\Program Files\xp2p")
CLIENT_CONFIG_DIR = win_env.CONFIG_ROOT / "config-client"
SERVER_CONFIG_DIR = win_env.CONFIG_ROOT / "config-server"
CLIENT_SERVICE_LOG = win_env.LOGS_DIR / "client" / "service.log"
SERVER_SERVICE_LOG = win_env.LOGS_DIR / "server" / "service.log"
CLIENT_INBOUNDS = CLIENT_CONFIG_DIR / "inbounds.json"
SERVER_INBOUNDS = SERVER_CONFIG_DIR / "inbounds.json"
CLIENT_CONFIG_FILE = win_env.CONFIG_ROOT / "xp2p-client.toml"
SERVER_CONFIG_FILE = win_env.CONFIG_ROOT / "xp2p-server.toml"
CLIENT_TUN = "xp2pc"
SERVER_TUN = "xp2ps"

SERVICE_TIMEOUT = 90.0
POLL_INTERVAL = 2.0


@contextmanager
def timed(label: str):
    start = time.perf_counter()
    try:
        yield
    finally:
        elapsed = time.perf_counter() - start
        print(f"TIMING: {label}: {elapsed:.2f}s")


def wait_for_service_state_cli(runner, role: str, expected_active: bool) -> None:
    wait_label = f"wait service {role} -> {'active' if expected_active else 'inactive'}"
    start = time.perf_counter()
    deadline = time.time() + SERVICE_TIMEOUT
    last_stdout = ""
    last_stderr = ""
    while time.time() < deadline:
        result = runner(role, "service", "status")
        active = result.rc == 0
        last_stdout = result.stdout or ""
        last_stderr = result.stderr or ""
        if active == expected_active:
            elapsed = time.perf_counter() - start
            print(f"TIMING: {wait_label}: {elapsed:.2f}s")
            return
        time.sleep(POLL_INTERVAL)
    state = "active" if expected_active else "inactive"
    elapsed = time.perf_counter() - start
    print(f"TIMING: {wait_label} timeout: {elapsed:.2f}s")
    pytest.fail(
        f"xp2p {role} service did not reach {state} state.\nSTDOUT:\n{last_stdout}\nSTDERR:\n{last_stderr}"
    )


def wait_for_log_entry(host, path: Path, phrase: str) -> None:
    start = time.perf_counter()
    deadline = time.time() + SERVICE_TIMEOUT
    needle = phrase.lower()
    last_content = ""
    while time.time() < deadline:
        if win_env.path_exists(host, path):
            content = win_env.read_text(host, path)
            last_content = content
            if needle in (content or "").lower():
                elapsed = time.perf_counter() - start
                print(f"TIMING: wait log '{phrase}': {elapsed:.2f}s")
                return
        time.sleep(POLL_INTERVAL)
    elapsed = time.perf_counter() - start
    print(f"TIMING: wait log '{phrase}' timeout: {elapsed:.2f}s")
    pytest.fail(f"Log {path} did not contain {phrase!r}. Last content:\n{last_content}")


def wait_for_log_entry_any(host, path: Path, phrases: list[str]) -> None:
    start = time.perf_counter()
    deadline = time.time() + SERVICE_TIMEOUT
    needles = [phrase.lower() for phrase in phrases]
    last_content = ""
    while time.time() < deadline:
        if win_env.path_exists(host, path):
            content = win_env.read_text(host, path)
            last_content = content
            lowered = (content or "").lower()
            if any(needle in lowered for needle in needles):
                elapsed = time.perf_counter() - start
                print(f"TIMING: wait log any ({len(phrases)}): {elapsed:.2f}s")
                return
        time.sleep(POLL_INTERVAL)
    phrase_list = ", ".join(repr(p) for p in phrases)
    elapsed = time.perf_counter() - start
    print(f"TIMING: wait log any timeout: {elapsed:.2f}s")
    pytest.fail(f"Log {path} did not contain any of {phrase_list}. Last content:\n{last_content}")


def write_text_utf8(host, path: Path, text: str) -> None:
    encoded = base64.b64encode(text.encode("utf-8")).decode("ascii")
    target = win_env.ps_quote(str(path))
    payload = win_env.ps_quote(encoded)
    script = f"""
$ErrorActionPreference = 'Stop'
$target = {target}
$payload = {payload}
$bytes = [System.Convert]::FromBase64String($payload)
$text = [System.Text.Encoding]::UTF8.GetString($bytes)
$dir = Split-Path -Parent $target
if ($dir -and -not (Test-Path $dir)) {{
    New-Item -ItemType Directory -Path $dir -Force | Out-Null
}}
$encoding = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText($target, $text, $encoding)
exit 0
"""
    result = win_env.run_powershell(host, script, label="write_text_utf8")
    if result.rc != 0:
        pytest.fail(
            "Failed to write config text.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def wait_for_apply_request_clear(host, timeout: float = 90.0) -> None:
    start = time.perf_counter()
    try:
        apply_flow.wait_for_apply_request_clear(
            host,
            timeout=timeout,
            poll_seconds=POLL_INTERVAL,
        )
    except Exception:  # noqa: BLE001
        elapsed = time.perf_counter() - start
        print(f"TIMING: wait apply.request clear timeout: {elapsed:.2f}s")
        raise
    elapsed = time.perf_counter() - start
    print(f"TIMING: wait apply.request clear: {elapsed:.2f}s")


def assert_ipv6_binding_disabled(host, interface_name: str) -> None:
    adapter_name, _adapter_index = tun_helpers.wait_for_tun_adapter(host, interface_name)
    result = win_env.run_guest_script(
        host,
        "scripts/assert_ipv6_binding_disabled.ps1",
        InterfaceName=adapter_name,
        TimeoutSeconds=120,
        PollSeconds=2,
    )
    if result.rc != 0:
        dump_path = win_env.dump_failure_state(host, label=f"ipv6-binding-{adapter_name}")
        pytest.fail(
            "IPv6 binding check failed.\n"
            f"Failure dump: {dump_path}\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def current_mode(host, role: str) -> str:
    path = CLIENT_CONFIG_FILE if role == "client" else SERVER_CONFIG_FILE
    state = win_env.read_toml(host, path).get(role) or {}
    tun_enabled = state.get("tun_enabled")
    if not isinstance(tun_enabled, bool):
        raise AssertionError(f"Expected tun_enabled boolean in {role} config, got {tun_enabled!r}")
    return "tun" if tun_enabled else "proxy"


def set_mode(runner, role: str, mode: str) -> None:
    runner(
        role,
        "mode",
        mode,
        "--path",
        str(INSTALL_ROOT),
        "--config-dir",
        f"config-{role}",
        check=True,
    )


def toggle_mode(host, runner, role: str) -> str:
    previous = current_mode(host, role)
    target = "proxy" if previous == "tun" else "tun"
    set_mode(runner, role, target)
    return previous


def cleanup_role(
    host,
    role: str,
    *,
    remove_config: bool,
    log_paths: list[Path] | None = None,
) -> None:
    parameters: dict[str, object] = {
        "Xp2pPath": str(win_env.XP2P_EXE),
        "Role": role,
        "InstallRoot": str(INSTALL_ROOT),
        "ConfigDir": f"config-{role}",
        "RemoveConfig": str(remove_config).lower(),
    }
    if log_paths:
        payload = base64.b64encode(json.dumps([str(path) for path in log_paths]).encode("utf-8")).decode(
            "ascii"
        )
        parameters["LogPathsBase64"] = payload
    result = win_env.run_guest_script(
        host,
        "scripts/xp2p_service_cleanup.ps1",
        **parameters,
    )
    if result.rc != 0:
        pytest.fail(
            "Failed to cleanup xp2p service state.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def require_service_installed(host, role: str) -> None:
    service_name = f"xp2p-{role}"
    if not win_env.service_exists(host, service_name):
        pytest.skip(f"{service_name} service is not registered; MSI-based install is required.")


def install_client(runner, host: str, user: str, password: str) -> None:
    runner(
        "client",
        "install",
        "--path",
        str(INSTALL_ROOT),
        "--config-dir",
        "config-client",
        "--host",
        host,
        "--user",
        user,
        "--password",
        password,
        "--force",
        check=True,
    )


def install_server(runner, host: str, port: str) -> None:
    runner(
        "server",
        "install",
        "--path",
        str(INSTALL_ROOT),
        "--config-dir",
        "config-server",
        "--host",
        host,
        "--port",
        port,
        "--force",
        check=True,
    )

