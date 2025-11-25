from __future__ import annotations

import time

import pytest

from tests.host.linux import _helpers as helpers

CLIENT_SERVICE_LOG = helpers.LOG_ROOT / "client" / "service.log"
CLIENT_XRAY_LOG = helpers.LOG_ROOT / "client" / "xray-service.log"
SERVER_SERVICE_LOG = helpers.LOG_ROOT / "server" / "service.log"
SERVER_XRAY_LOG = helpers.LOG_ROOT / "server" / "xray-service.log"
CLIENT_INBOUNDS = helpers.CLIENT_CONFIG_DIR / "inbounds.json"
SERVER_INBOUNDS = helpers.SERVER_CONFIG_DIR / "inbounds.json"

SERVICE_TIMEOUT = 45.0
POLL_INTERVAL = 1.5


def _wait_for_service_state(runner, role: str, expected_active: bool) -> None:
    deadline = time.time() + SERVICE_TIMEOUT
    last_result = None
    while time.time() < deadline:
        result = runner(role, "service", "status")
        active = result.rc == 0
        if active == expected_active:
            return
        last_result = result
        time.sleep(POLL_INTERVAL)

    state = "active" if expected_active else "inactive"
    stdout = (last_result.stdout or "") if last_result else ""
    stderr = (last_result.stderr or "") if last_result else ""
    raise AssertionError(
        f"xp2p {role} service did not reach {state} state. "
        f"Last rc: {getattr(last_result, 'rc', 'n/a')}\nSTDOUT:\n{stdout}\nSTDERR:\n{stderr}"
    )


def _install_client_endpoint(runner, host_addr: str, user: str, password: str, *, sni: str | None = None) -> None:
    args = [
        "client",
        "install",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
        "--host",
        host_addr,
        "--user",
        user,
        "--password",
        password,
        "--force",
    ]
    if sni:
        args.extend(["--sni", sni])
    runner(*args, check=True)


def _install_server_instance(runner, host_name: str = "svc-server.example.com", port: str = "62120") -> None:
    runner(
        "server",
        "install",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.SERVER_CONFIG_DIR_NAME,
        "--port",
        port,
        "--host",
        host_name,
        "--force",
        check=True,
    )


def _start_service(role: str, runner) -> None:
    runner(role, "service", "stop")
    runner(role, "service", "start", check=True)
    _wait_for_service_state(runner, role, expected_active=True)


def _stop_service(role: str, runner) -> None:
    runner(role, "service", "stop")
    _wait_for_service_state(runner, role, expected_active=False)


def _clear_logs(host, *paths) -> None:
    for path in paths:
        helpers.remove_path(host, path)


def _wait_for_log_entry(host, path, substring: str, timeout: float = 60.0) -> None:
    deadline = time.time() + timeout
    lowered = substring.lower()
    last_content = ""
    while time.time() < deadline:
        if helpers.path_exists(host, path):
            content = helpers.read_text(host, path)
            last_content = content
            if lowered in content.lower():
                return
        time.sleep(POLL_INTERVAL)
    raise AssertionError(f"Log {path} did not contain {substring!r}. Last content:\n{last_content}")


@pytest.mark.host
@pytest.mark.linux
def test_client_service_cli_controls_systemd(client_host, xp2p_client_runner):
    helpers.cleanup_client_install(client_host, xp2p_client_runner)
    try:
        _install_client_endpoint(
            xp2p_client_runner,
            "10.55.10.50",
            "svc-client@example.com",
            "svc-client-secret",
        )

        _clear_logs(client_host, CLIENT_SERVICE_LOG, CLIENT_XRAY_LOG)
        _start_service("client", xp2p_client_runner)
        assert helpers.path_exists(client_host, CLIENT_SERVICE_LOG), "client service log was not created"

        _stop_service("client", xp2p_client_runner)
        assert helpers.path_exists(client_host, CLIENT_SERVICE_LOG), "client service log missing after stop"
    finally:
        _stop_service("client", xp2p_client_runner)
        helpers.cleanup_client_install(client_host, xp2p_client_runner)
        _clear_logs(client_host, CLIENT_SERVICE_LOG, CLIENT_XRAY_LOG)


@pytest.mark.host
@pytest.mark.linux
def test_server_service_cli_controls_systemd(server_host, xp2p_server_runner):
    helpers.cleanup_server_install(server_host, xp2p_server_runner)
    try:
        _install_server_instance(xp2p_server_runner, host_name="svc-server.example.com", port="62120")

        _clear_logs(server_host, SERVER_SERVICE_LOG, SERVER_XRAY_LOG)
        _start_service("server", xp2p_server_runner)
        assert helpers.path_exists(server_host, SERVER_SERVICE_LOG), "server service log was not created"

        _stop_service("server", xp2p_server_runner)
        assert helpers.path_exists(server_host, SERVER_SERVICE_LOG), "server service log missing after stop"
    finally:
        _stop_service("server", xp2p_server_runner)
        helpers.cleanup_server_install(server_host, xp2p_server_runner)
        _clear_logs(server_host, SERVER_SERVICE_LOG, SERVER_XRAY_LOG)


@pytest.mark.host
@pytest.mark.linux
@pytest.mark.parametrize("role", ["client", "server"])
def test_service_restarts_when_config_changes(
    role,
    client_host,
    server_host,
    xp2p_client_runner,
    xp2p_server_runner,
):
    if role == "client":
        host = client_host
        runner = xp2p_client_runner
        cleanup = helpers.cleanup_client_install
        service_log = CLIENT_SERVICE_LOG
        xray_log = CLIENT_XRAY_LOG
        base_host = "10.55.10.60"
        redirect_cidr = "198.18.0.0/15"
        install_fn = lambda: _install_client_endpoint(
            xp2p_client_runner,
            base_host,
            "svc-change@example.com",
            "svc-change-secret",
        )
        change_fn = lambda: xp2p_client_runner(
            "client",
            "redirect",
            "add",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--cidr",
            redirect_cidr,
            "--host",
            base_host,
            check=True,
        )
        revert_fn = lambda: xp2p_client_runner(
            "client",
            "redirect",
            "remove",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--cidr",
            redirect_cidr,
            "--host",
            base_host,
            check=True,
        )
    else:
        host = server_host
        runner = xp2p_server_runner
        cleanup = helpers.cleanup_server_install
        service_log = SERVER_SERVICE_LOG
        xray_log = SERVER_XRAY_LOG
        install_fn = lambda: _install_server_instance(
            xp2p_server_runner,
            host_name="svc-server.example.com",
            port="62125",
        )
        user_id = "svc-restart@example.com"

        def change_fn():
            xp2p_server_runner(
                "server",
                "user",
                "add",
                "--path",
                helpers.INSTALL_ROOT.as_posix(),
                "--config-dir",
                helpers.SERVER_CONFIG_DIR_NAME,
                "--id",
                user_id,
                "--password",
                "SvcRestart123",
                "--host",
                "svc-server.example.com",
                check=True,
            )

        def revert_fn():
            xp2p_server_runner(
                "server",
                "user",
                "remove",
                "--path",
                helpers.INSTALL_ROOT.as_posix(),
                "--config-dir",
                helpers.SERVER_CONFIG_DIR_NAME,
                "--id",
                user_id,
                "--host",
                "svc-server.example.com",
                check=True,
            )

    cleanup(host, runner)
    try:
        install_fn()
        _clear_logs(host, service_log, xray_log)
        _start_service(role, runner)

        change_applied = False
        try:
            change_fn()
            change_applied = True
        finally:
            if change_applied:
                revert_fn()

        _wait_for_log_entry(host, service_log, "service configuration change detected")
        _wait_for_service_state(runner, role, expected_active=True)
    finally:
        _stop_service(role, runner)
        cleanup(host, runner)
        _clear_logs(host, service_log, xray_log)


@pytest.mark.host
@pytest.mark.linux
@pytest.mark.parametrize("role", ["client", "server"])
def test_service_stops_after_invalid_config(
    role,
    client_host,
    server_host,
    xp2p_client_runner,
    xp2p_server_runner,
):
    max_attempts = 5
    if role == "client":
        host = client_host
        runner = xp2p_client_runner
        cleanup = helpers.cleanup_client_install
        service_log = CLIENT_SERVICE_LOG
        xray_log = CLIENT_XRAY_LOG
        config_path = CLIENT_INBOUNDS
        install_fn = lambda: _install_client_endpoint(
            xp2p_client_runner,
            "10.55.10.70",
            "svc-fail@example.com",
            "svc-fail-secret",
        )
    else:
        host = server_host
        runner = xp2p_server_runner
        cleanup = helpers.cleanup_server_install
        service_log = SERVER_SERVICE_LOG
        xray_log = SERVER_XRAY_LOG
        config_path = SERVER_INBOUNDS
        install_fn = lambda: _install_server_instance(
            xp2p_server_runner,
            host_name="svc-server.example.com",
            port="62130",
        )

    cleanup(host, runner)
    try:
        install_fn()
        _clear_logs(host, service_log, xray_log)

        runner(role, "service", "stop")
        helpers.write_text(host, config_path, "INVALID-CONFIG")
        runner(role, "service", "start")

        _wait_for_log_entry(host, service_log, "exceeded restart limit")
        _wait_for_service_state(runner, role, expected_active=False)

        log_content = helpers.read_text(host, service_log)
        attempts = log_content.lower().count("attempt:")
        assert attempts >= max_attempts, f"expected at least {max_attempts} restart attempts"
        assert helpers.path_exists(host, xray_log), f"{role} xray log missing"
        assert helpers.read_text(host, xray_log).strip(), f"{role} xray log is empty"
    finally:
        runner(role, "service", "stop")
        cleanup(host, runner)
        _clear_logs(host, service_log, xray_log)
