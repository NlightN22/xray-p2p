from __future__ import annotations

import base64
import json
import time
from contextlib import contextmanager
from pathlib import Path

import pytest

from tests.host.win import env as win_env

INSTALL_ROOT = Path(r"C:\Program Files\xp2p")
CLIENT_CONFIG_DIR = win_env.CONFIG_ROOT / "config-client"
SERVER_CONFIG_DIR = win_env.CONFIG_ROOT / "config-server"
CLIENT_SERVICE_LOG = win_env.LOGS_DIR / "client" / "service.log"
CLIENT_XRAY_LOG = win_env.LOGS_DIR / "client" / "xray-service.log"
SERVER_SERVICE_LOG = win_env.LOGS_DIR / "server" / "service.log"
SERVER_XRAY_LOG = win_env.LOGS_DIR / "server" / "xray-service.log"
CLIENT_INBOUNDS = CLIENT_CONFIG_DIR / "inbounds.json"
SERVER_INBOUNDS = SERVER_CONFIG_DIR / "inbounds.json"
CLIENT_CONFIG_FILE = win_env.CONFIG_ROOT / "xp2p-client.toml"
SERVER_CONFIG_FILE = win_env.CONFIG_ROOT / "xp2p-server.toml"
CLIENT_TUN = "xp2pc"
SERVER_TUN = "xp2ps"

SERVICE_TIMEOUT = 90.0
POLL_INTERVAL = 2.0


@contextmanager
def _timed(label: str):
    start = time.perf_counter()
    try:
        yield
    finally:
        elapsed = time.perf_counter() - start
        print(f"TIMING: {label}: {elapsed:.2f}s")


def _wait_for_service_state(runner, role: str, expected_active: bool) -> None:
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


def _wait_for_log_entry(host, path: Path, phrase: str) -> None:
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


def _wait_for_log_entry_any(host, path: Path, phrases: list[str]) -> None:
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


def _wait_for_log_nonempty(host, path: Path, label: str) -> str:
    start = time.perf_counter()
    deadline = time.time() + SERVICE_TIMEOUT
    last_content = ""
    while time.time() < deadline:
        if win_env.path_exists(host, path):
            content = win_env.read_text(host, path)
            last_content = content
            if (content or "").strip():
                elapsed = time.perf_counter() - start
                print(f"TIMING: wait log nonempty ({label}): {elapsed:.2f}s")
                return content
        time.sleep(POLL_INTERVAL)
    elapsed = time.perf_counter() - start
    print(f"TIMING: wait log nonempty ({label}) timeout: {elapsed:.2f}s")
    pytest.fail(f"Log {path} remained empty for {label}. Last content:\n{last_content}")


def _assert_ipv6_binding_disabled(host, interface_name: str) -> None:
    result = win_env.run_guest_script(
        host,
        "scripts/assert_ipv6_binding_disabled.ps1",
        InterfaceName=interface_name,
        TimeoutSeconds=60,
        PollSeconds=2,
    )
    if result.rc != 0:
        pytest.fail(
            "IPv6 binding check failed.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


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


def _cleanup_role(
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
        payload = base64.b64encode(
            json.dumps([str(path) for path in log_paths]).encode("utf-8")
        ).decode("ascii")
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


def _require_service_installed(host, role: str) -> None:
    service_name = f"xp2p-{role}"
    if not win_env.service_exists(host, service_name):
        pytest.skip(
            f"{service_name} service is not registered; MSI-based install is required."
        )


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
    with _timed("clear client logs"):
        _cleanup_role(
            client_host,
            "client",
            remove_config=False,
            log_paths=[CLIENT_SERVICE_LOG, CLIENT_XRAY_LOG],
        )

    with _timed("client install"):
        _install_client(runner, "10.70.0.10", "svc-client@example.com", "SvcClientSecret")
    _require_service_installed(client_host, "client")
    with _timed("client service start"):
        runner("client", "service", "start", check=True)
    _wait_for_service_state(runner, "client", expected_active=True)
    if _current_mode(client_host, "client") == "tun":
        _assert_ipv6_binding_disabled(client_host, CLIENT_TUN)
    assert win_env.path_exists(client_host, CLIENT_SERVICE_LOG), "client service log not created"

    with _timed("client service stop (final)"):
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

    with _timed(f"{role} cleanup (pre)"):
        _cleanup_role(
            host,
            role,
            remove_config=True,
            log_paths=[log_path, xray_log],
        )
    with _timed(f"{role} install"):
        install_fn()
    _require_service_installed(host, role)
    try:
        with _timed(f"{role} clear logs"):
            _cleanup_role(
                host,
                role,
                remove_config=False,
                log_paths=[log_path, xray_log],
            )
        with _timed(f"{role} service start"):
            runner(role, "service", "start", check=True)
        _wait_for_service_state(runner, role, expected_active=True)
        if _current_mode(host, role) == "tun":
            if role == "client":
                _assert_ipv6_binding_disabled(host, CLIENT_TUN)
            else:
                _assert_ipv6_binding_disabled(host, SERVER_TUN)

        with _timed(f"{role} change config"):
            change_fn()
        with _timed(f"{role} revert config"):
            revert_fn()

        _wait_for_log_entry(host, log_path, "service configuration change detected")
        _wait_for_service_state(runner, role, expected_active=True)
    finally:
        with _timed(f"{role} cleanup (final)"):
            _cleanup_role(
                host,
                role,
                remove_config=True,
                log_paths=None,
            )


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
        config_path = CLIENT_CONFIG_FILE
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
        config_path = SERVER_CONFIG_FILE
        install_fn = lambda: _install_server(
            runner,
            "svc-fail.example.com",
            "62190",
        )

    with _timed(f"{role} cleanup (pre)"):
        _cleanup_role(
            host,
            role,
            remove_config=True,
            log_paths=[log_path, xray_log],
        )
    with _timed(f"{role} install"):
        install_fn()
    _require_service_installed(host, role)
    try:
        with _timed(f"{role} clear logs"):
            _cleanup_role(
                host,
                role,
                remove_config=False,
                log_paths=[log_path, xray_log],
            )
        with _timed(f"{role} service start"):
            runner(role, "service", "start", check=True)
        _wait_for_service_state(runner, role, expected_active=True)

        with _timed(f"{role} write broken config"):
            win_env.write_text(host, config_path, "BROKEN-CONFIG")
        _wait_for_log_entry_any(
            host,
            log_path,
            [
                "exceeded restart limit",
                "service run failed",
                "service failed",
            ],
        )
        _wait_for_service_state(runner, role, expected_active=False)
        assert win_env.path_exists(host, xray_log), f"{role} xray log missing"
        if role == "client":
            _wait_for_log_nonempty(host, xray_log, f"{role} xray")
    finally:
        with _timed(f"{role} cleanup (final)"):
            _cleanup_role(
                host,
                role,
                remove_config=True,
                log_paths=None,
            )
