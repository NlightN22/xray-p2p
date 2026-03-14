from __future__ import annotations

import base64
import json
import os
import uuid
from pathlib import Path

import pytest

from tests.host import common
from tests.host.win import env as win_env

INSTALL_ROOT = Path(r"C:\Program Files\xp2p")
UI_EXE = INSTALL_ROOT / "xp2p-ui.exe"
MARKER_DIR = Path(r"C:\xp2p\build\ui-markers")
LOCAL_MARKER_DIR = common.REPO_ROOT / "build" / "ui-markers"


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
    local_marker, guest_marker = _marker_paths("xp2p-ui-install")
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
            "xp2p-ui install/autostart check failed.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


@pytest.mark.host
@pytest.mark.win
def test_xp2p_ui_smoke_launch(server_host):
    local_marker, guest_marker = _marker_paths("xp2p-ui-launch")
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
            "xp2p-ui failed to launch or exit cleanly.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


@pytest.mark.host
@pytest.mark.win
def test_xp2p_ui_logs_do_not_report_access_denied(server_host):
    local_marker, guest_marker = _marker_paths("xp2p-ui-logs")
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
            "xp2p-ui log check failed.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        ) 


@pytest.mark.host
@pytest.mark.win
def test_sc_query_as_non_admin_user(server_host):
    local_marker, guest_marker = _marker_paths("xp2p-ui-sc-query")
    result = win_env.run_guest_script(
        server_host,
        "scripts/check_sc_query_as_user.ps1",
        MarkerPath=str(guest_marker),
        ServiceNames="xp2p-client,xp2p-server",
        UserName="vagrant",
        UserPassword="vagrant",
        UseExistingUser="1",
        GrantLogonRights="0",
    )
    _assert_marker(local_marker, "check_sc_query_as_user.ps1")
    if result.rc != 0:
        pytest.fail(
            "sc query as non-admin user failed.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


@pytest.mark.host
@pytest.mark.win
def test_xp2p_ui_controls_services_without_admin(server_host, xp2p_server_runner):
    services = ["xp2p-client", "xp2p-server"]
    missing = [name for name in services if not win_env.service_exists(server_host, name)]
    if missing:
        pytest.skip(f"Services not registered: {', '.join(missing)}")

    _install_client(xp2p_server_runner)
    _install_server(xp2p_server_runner)
    local_marker, guest_marker = _marker_paths("xp2p-ui-service-toggle")
    payload = base64.b64encode(json.dumps(services).encode("utf-8")).decode("ascii")
    try:
        result = win_env.run_guest_script(
            server_host,
            "scripts/toggle_service_via_ui.ps1",
            MarkerPath=str(guest_marker),
            Xp2pUiPath=str(UI_EXE),
            ServiceNamesBase64=payload,
            UiWaitSeconds="5",
            ServiceWaitSeconds="30",
        )
        _assert_marker(local_marker, "toggle_service_via_ui.ps1")
        if result.rc != 0:
            pytest.fail(
                "xp2p-ui service toggle check failed.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
    finally:
        _cleanup_role(server_host, "client")
        _cleanup_role(server_host, "server")
