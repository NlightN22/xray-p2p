from __future__ import annotations

import time
from pathlib import Path

import pytest

from tests.host.win import env as win_env

INSTALL_ROOT = Path(r"C:\Program Files\xp2p")
CLIENT_CONFIG_DIR = INSTALL_ROOT / "config-client"
SERVER_CONFIG_DIR = INSTALL_ROOT / "config-server"
CLIENT_SERVICE_LOG = INSTALL_ROOT / "logs" / "client" / "service.log"
CLIENT_XRAY_LOG = INSTALL_ROOT / "logs" / "client" / "xray-service.log"
SERVER_SERVICE_LOG = INSTALL_ROOT / "logs" / "server" / "service.log"
SERVER_XRAY_LOG = INSTALL_ROOT / "logs" / "server" / "xray-service.log"
CLIENT_INBOUNDS = CLIENT_CONFIG_DIR / "inbounds.json"
SERVER_INBOUNDS = SERVER_CONFIG_DIR / "inbounds.json"

SERVICE_TIMEOUT = 90.0
POLL_INTERVAL = 2.0


def _wait_for_service_state(runner, role: str, expected_active: bool) -> None:
    deadline = time.time() + SERVICE_TIMEOUT
    last_stdout = ""
    last_stderr = ""
    while time.time() < deadline:
        result = runner(role, "service", "status")
        active = result.rc == 0
        last_stdout = result.stdout or ""
        last_stderr = result.stderr or ""
        if active == expected_active:
            return
        time.sleep(POLL_INTERVAL)
    state = "active" if expected_active else "inactive"
    pytest.fail(
        f"xp2p {role} service did not reach {state} state.\nSTDOUT:\n{last_stdout}\nSTDERR:\n{last_stderr}"
    )


def _wait_for_log_entry(host, path: Path, phrase: str) -> None:
    deadline = time.time() + SERVICE_TIMEOUT
    needle = phrase.lower()
    last_content = ""
    while time.time() < deadline:
        if win_env.path_exists(host, path):
            content = win_env.read_text(host, path)
            last_content = content
            if needle in (content or "").lower():
                return
        time.sleep(POLL_INTERVAL)
    pytest.fail(f"Log {path} did not contain {phrase!r}. Last content:\n{last_content}")


def _current_mode(host, role: str) -> str:
    path = CLIENT_CONFIG_FILE if role == "client" else SERVER_CONFIG_FILE
    state = win_env.read_toml(host, path).get(role) or {}
    tun_enabled = state.get("tun_enabled")
    if not isinstance(tun_enabled, bool):
        raise AssertionError(f"Expected tun_enabled boolean in {role} config, got {tun_enabled!r}")
    return "tun" if tun_enabled else "proxy"


def _set_mode(runner, role: str, mode: str) -> None:
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


def _toggle_mode(host, runner, role: str) -> str:
    previous = _current_mode(host, role)
    target = "proxy" if previous == "tun" else "tun"
    _set_mode(runner, role, target)
    return previous


def _clear_paths(host, *paths: Path) -> None:
    for path in paths:
        win_env.remove_path(host, path)


def _install_client(runner, host: str, user: str, password: str) -> None:
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


def _install_server(runner, host: str, port: str) -> None:
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


@pytest.mark.host
def test_windows_client_service_cli_controls_service(client_host, xp2p_client_runner):
    runner = xp2p_client_runner
    _clear_paths(client_host, CLIENT_SERVICE_LOG, CLIENT_XRAY_LOG)
    runner("client", "service", "stop")

    _install_client(runner, "10.70.0.10", "svc-client@example.com", "SvcClientSecret")
    runner("client", "service", "start", check=True)
    _wait_for_service_state(runner, "client", expected_active=True)
    assert win_env.path_exists(client_host, CLIENT_SERVICE_LOG), "client service log not created"

    runner("client", "service", "stop", check=True)
    _wait_for_service_state(runner, "client", expected_active=False)


@pytest.mark.host
@pytest.mark.parametrize("role", ["client", "server"])
def test_windows_service_restarts_when_config_changes(
    role,
    client_host,
    server_host,
    xp2p_client_runner,
    xp2p_server_runner,
):
    if role == "client":
        host = client_host
        runner = xp2p_client_runner
        log_path = CLIENT_SERVICE_LOG
        xray_log = CLIENT_XRAY_LOG
        original_mode: str | None = None
        install_fn = lambda: _install_client(
            runner,
            "10.70.0.20",
            "svc-change@example.com",
            "SvcChangeSecret",
        )

        def change_fn():
            nonlocal original_mode
            original_mode = _toggle_mode(host, runner, role)

        def revert_fn():
            if original_mode:
                _set_mode(runner, role, original_mode)

    else:
        host = server_host
        runner = xp2p_server_runner
        log_path = SERVER_SERVICE_LOG
        xray_log = SERVER_XRAY_LOG
        original_mode: str | None = None
        install_fn = lambda: _install_server(
            runner,
            "svc-server.example.com",
            "62180",
        )

        def change_fn():
            nonlocal original_mode
            original_mode = _toggle_mode(host, runner, role)

        def revert_fn():
            if original_mode:
                _set_mode(runner, role, original_mode)

    runner(role, "remove", "--path", str(INSTALL_ROOT), "--config-dir", f"config-{role}")
    install_fn()
    try:
        _clear_paths(host, log_path, xray_log)
        runner(role, "service", "stop")
        runner(role, "service", "start", check=True)
        _wait_for_service_state(runner, role, expected_active=True)

        change_fn()
        revert_fn()

        _wait_for_log_entry(host, log_path, "service configuration change detected")
        _wait_for_service_state(runner, role, expected_active=True)
    finally:
        runner(role, "service", "stop")
        runner(role, "remove", "--path", str(INSTALL_ROOT), "--config-dir", f"config-{role}")


@pytest.mark.host
@pytest.mark.parametrize("role", ["client", "server"])
def test_windows_service_stops_after_invalid_config(
    role,
    client_host,
    server_host,
    xp2p_client_runner,
    xp2p_server_runner,
):
    if role == "client":
        host = client_host
        runner = xp2p_client_runner
        log_path = CLIENT_SERVICE_LOG
        xray_log = CLIENT_XRAY_LOG
        config_path = CLIENT_INBOUNDS
        install_fn = lambda: _install_client(
            runner,
            "10.70.0.30",
            "svc-fail@example.com",
            "SvcFailSecret",
        )
    else:
        host = server_host
        runner = xp2p_server_runner
        log_path = SERVER_SERVICE_LOG
        xray_log = SERVER_XRAY_LOG
        config_path = SERVER_INBOUNDS
        install_fn = lambda: _install_server(
            runner,
            "svc-fail.example.com",
            "62190",
        )

    runner(role, "remove", "--path", str(INSTALL_ROOT), "--config-dir", f"config-{role}")
    install_fn()
    try:
        _clear_paths(host, log_path, xray_log)
        runner(role, "service", "stop")
        win_env.write_text(host, config_path, "BROKEN-CONFIG")
        runner(role, "service", "start")

        _wait_for_log_entry(host, log_path, "exceeded restart limit")
        _wait_for_service_state(runner, role, expected_active=False)
        assert win_env.path_exists(host, xray_log), f"{role} xray log missing"
        assert win_env.read_text(host, xray_log).strip(), f"{role} xray log is empty"
    finally:
        runner(role, "service", "stop")
        runner(role, "remove", "--path", str(INSTALL_ROOT), "--config-dir", f"config-{role}")
