from __future__ import annotations

import base64
import json
import os
import time
import uuid
from pathlib import Path

import pytest

from tests.host import common
from tests.host.win import env as win_env

INSTALL_ROOT = Path(r"C:\Program Files\xp2p")
UI_EXE = INSTALL_ROOT / "ui-xp2p.exe"
MARKER_DIR = Path(r"C:\xp2p\build\ui-markers")
LOCAL_MARKER_DIR = common.REPO_ROOT / "build" / "ui-markers"


def _require_ui_installed(host) -> None:
    if not win_env.path_exists(host, UI_EXE):
        pytest.skip(f"ui-xp2p.exe not found at {UI_EXE}")


def _autostart_expected() -> str:
    raw = os.environ.get("XP2P_UI_AUTOSTART")
    if raw is None or raw.strip() == "":
        return "true"
    normalized = raw.strip().lower()
    if normalized in {"1", "true", "yes"}:
        return "true"
    if normalized in {"0", "false", "no"}:
        return "false"
    pytest.fail(f"Invalid XP2P_UI_AUTOSTART value: {raw!r}")


def _marker_paths(label: str) -> tuple[Path, Path]:
    token = uuid.uuid4().hex
    name = f"{label}-{token}.txt"
    local = LOCAL_MARKER_DIR / name
    guest = MARKER_DIR / name
    LOCAL_MARKER_DIR.mkdir(parents=True, exist_ok=True)
    if local.exists():
        local.unlink()
    return local, guest


def _assert_marker(local_path: Path, script_name: str) -> None:
    if not local_path.exists():
        pytest.fail(f"{script_name} did not create marker at {local_path}")
    payload = local_path.read_text(encoding="ascii", errors="ignore").strip()
    if not payload.startswith("OK"):
        pytest.fail(f"{script_name} marker indicates failure: {payload or '<empty>'}")


def _read_optional_scesrv_log_tail(host, *, lines: int = 200) -> str:
    script = f"""
$ErrorActionPreference = 'SilentlyContinue'
$path = Join-Path $env:windir 'security\\logs\\scesrv.log'
if (-not (Test-Path $path)) {{
    exit 3
}}
Get-Content -Path $path -Tail {lines} -ErrorAction SilentlyContinue
exit 0
"""
    result = win_env.run_powershell(host, script, timeout=30, label="read_scesrv_log_tail")
    if result.rc != 0:
        return ""
    return (result.stdout or "").strip()


def _run_guest_script_with_marker_retry(
    host,
    script_path: str,
    *,
    marker_label: str,
    attempts: int = 3,
    retry_sleep_seconds: float = 3.0,
    **kwargs: str,
):
    last_result = None
    last_payload = ""
    for attempt in range(1, attempts + 1):
        local_marker, guest_marker = _marker_paths(marker_label)
        result = win_env.run_guest_script(host, script_path, MarkerPath=str(guest_marker), **kwargs)
        last_result = result
        if not local_marker.exists():
            pytest.fail(f"{script_path} did not create marker at {local_marker}")
        payload = local_marker.read_text(encoding="ascii", errors="ignore").strip()
        last_payload = payload
        if payload.startswith("OK"):
            return result

        normalized = payload.lower()
        secedit_error = "secedit configure failed" in normalized
        if secedit_error:
            if attempt < attempts:
                print(
                    f"WARNING: {script_path} failed due to secedit error (attempt {attempt}/{attempts}), retrying."
                )
                time.sleep(retry_sleep_seconds * attempt)
                continue

            scesrv_tail = _read_optional_scesrv_log_tail(host)
            message = (
                f"Skipping UI integration step because secedit failed while preparing user rights: {payload or '<empty>'}"
            )
            if scesrv_tail:
                message += "\n\nscesrv.log tail:\n" + scesrv_tail
            pytest.skip(message)

        scesrv_tail = _read_optional_scesrv_log_tail(host)
        details = f"{script_path} marker indicates failure: {payload or '<empty>'}"
        if scesrv_tail:
            details += "\n\nscesrv.log tail:\n" + scesrv_tail
        if last_result is not None:
            details += f"\n\nSTDOUT:\n{last_result.stdout}\nSTDERR:\n{last_result.stderr}"
        pytest.fail(details)

    assert last_result is not None
    pytest.fail(f"{script_path} marker indicates failure: {last_payload or '<empty>'}")


def _cleanup_role(host, role: str) -> None:
    win_env.run_guest_script(
        host,
        "scripts/xp2p_service_cleanup.ps1",
        Xp2pPath=str(win_env.XP2P_EXE),
        Role=role,
        InstallRoot=str(INSTALL_ROOT),
        ConfigDir=f"config-{role}",
        RemoveConfig="true",
    )


def _install_client(runner) -> None:
    runner(
        "client",
        "install",
        "--path",
        str(INSTALL_ROOT),
        "--config-dir",
        "config-client",
        "--host",
        "10.80.0.10",
        "--user",
        "ui-client@example.com",
        "--password",
        "UiClientSecret",
        "--force",
        check=True,
    )


def _install_server(runner) -> None:
    runner(
        "server",
        "install",
        "--path",
        str(INSTALL_ROOT),
        "--config-dir",
        "config-server",
        "--host",
        "ui-server.example.com",
        "--port",
        "62155",
        "--force",
        check=True,
    )


@pytest.mark.host
@pytest.mark.win
def test_xp2p_ui_install_and_autostart(server_host):
    _require_ui_installed(server_host)
    local_marker, guest_marker = _marker_paths("ui-xp2p-install")
    result = win_env.run_guest_script(
        server_host,
        "scripts/check_xp2p_ui_install.ps1",
        Xp2pUiPath=str(UI_EXE),
        MarkerPath=str(guest_marker),
        AutostartExpected=_autostart_expected(),
    )
    _assert_marker(local_marker, "check_xp2p_ui_install.ps1")
    if result.rc != 0:
        pytest.fail(
            "ui-xp2p install/autostart check failed.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


@pytest.mark.host
@pytest.mark.win
def test_xp2p_ui_smoke_launch(server_host):
    _require_ui_installed(server_host)
    local_marker, guest_marker = _marker_paths("ui-xp2p-launch")
    result = win_env.run_guest_script(
        server_host,
        "scripts/launch_xp2p_ui.ps1",
        Xp2pUiPath=str(UI_EXE),
        MarkerPath=str(guest_marker),
        WaitSeconds="8",
    )
    _assert_marker(local_marker, "launch_xp2p_ui.ps1")
    if result.rc != 0:
        pytest.fail(
            "ui-xp2p failed to launch or exit cleanly.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


@pytest.mark.host
@pytest.mark.win
def test_xp2p_ui_logs_do_not_report_access_denied(server_host):
    _require_ui_installed(server_host)
    local_marker, guest_marker = _marker_paths("ui-xp2p-logs")
    result = win_env.run_guest_script(
        server_host,
        "scripts/check_xp2p_ui_logs.ps1",
        Xp2pUiPath=str(UI_EXE),
        MarkerPath=str(guest_marker),
        WaitSeconds="8",
        MaxLines="200",
        ClearLog="true",
    )
    _assert_marker(local_marker, "check_xp2p_ui_logs.ps1")
    if result.rc != 0:
        pytest.fail(
            "ui-xp2p log check failed.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


@pytest.mark.host
@pytest.mark.win
def test_sc_query_as_non_admin_user(server_host, xp2p_server_runner):
    _install_client(xp2p_server_runner)
    _install_server(xp2p_server_runner)
    try:
        result = _run_guest_script_with_marker_retry(
            server_host,
            "scripts/check_sc_query_as_user.ps1",
            marker_label="ui-xp2p-sc-query",
            ServiceNames="xp2p-client,xp2p-server",
            UserName="vagrant",
            UserPassword="vagrant",
            UseExistingUser="1",
            GrantLogonRights="0",
            )
        if result.rc != 0:
            pytest.fail(
                "sc query as non-admin user failed.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    finally:
        _cleanup_role(server_host, "client")
        _cleanup_role(server_host, "server")


@pytest.mark.host
@pytest.mark.win
def test_xp2p_ui_controls_services_without_admin(server_host, xp2p_server_runner):
    _require_ui_installed(server_host)
    services = ["xp2p-client", "xp2p-server"]
    missing = [name for name in services if not win_env.service_exists(server_host, name)]
    if missing:
        pytest.skip(f"Services not registered: {', '.join(missing)}")

    _install_client(xp2p_server_runner)
    _install_server(xp2p_server_runner)
    log_patterns = [
        r"tray status: .*Running",
        r"tray status: .*Stopped",
    ]
    log_patterns_payload = base64.b64encode(
        json.dumps(log_patterns).encode("utf-8")
    ).decode("ascii")
    payload = base64.b64encode(json.dumps(services).encode("utf-8")).decode("ascii")
    try:
        result = _run_guest_script_with_marker_retry(
            server_host,
            "scripts/toggle_service_via_ui.ps1",
            marker_label="ui-xp2p-service-toggle",
            Xp2pUiPath=str(UI_EXE),
            ServiceNamesBase64=payload,
            UiWaitSeconds="5",
            ServiceWaitSeconds="30",
            ClearLog="true",
            UiPollSeconds="6",
            RequiredPatternsBase64=log_patterns_payload,
            )
        if result.rc != 0:
            pytest.fail(
                "ui-xp2p service toggle check failed.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    finally:
        _cleanup_role(server_host, "client")
        _cleanup_role(server_host, "server")


@pytest.mark.host
@pytest.mark.win
def test_xp2p_ui_tracks_service_crash_without_config(server_host, xp2p_server_runner):
    _require_ui_installed(server_host)
    _install_client(xp2p_server_runner)
    config_paths = [
        r"C:\ProgramData\xp2p\config-client",
        r"C:\ProgramData\xp2p\xp2p-client.toml",
        r"C:\ProgramData\xp2p\xp2p-client.state.json",
    ]
    config_payload = base64.b64encode(json.dumps(config_paths).encode("utf-8")).decode("ascii")
    services_payload = base64.b64encode(json.dumps(["xp2p-client"]).encode("utf-8")).decode("ascii")
    try:
        result = _run_guest_script_with_marker_retry(
            server_host,
            "scripts/toggle_service_via_ui.ps1",
            marker_label="ui-xp2p-crash-track",
            Xp2pUiPath=str(UI_EXE),
            ServiceNamesBase64=services_payload,
            UiWaitSeconds="5",
            ServiceWaitSeconds="20",
            ClearLog="true",
            UiPollSeconds="6",
            ExpectCrash="true",
            CrashWaitSeconds="60",
            CrashServiceName="xp2p-client",
            ConfigDirsBase64=config_payload,
            StatusPollSeconds="1",
            AllowStoppedOnly="true",
            StartStatusesBase64=base64.b64encode(
                json.dumps(["StartPending", "Running"]).encode("utf-8")
            ).decode("ascii"),
            )
        if result.rc != 0:
            pytest.fail(
                "ui-xp2p crash tracking check failed.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    finally:
        _cleanup_role(server_host, "client")
