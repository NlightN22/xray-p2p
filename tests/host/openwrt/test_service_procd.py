from __future__ import annotations

import time
from pathlib import PurePosixPath

import pytest

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env

CLIENT_SERVICE_LOG = helpers.LOG_ROOT / "client" / "service.log"
CLIENT_XRAY_LOG = helpers.LOG_ROOT / "client" / "xray-service.log"
SERVER_SERVICE_LOG = helpers.LOG_ROOT / "server" / "service.log"
SERVER_XRAY_LOG = helpers.LOG_ROOT / "server" / "xray-service.log"
CLIENT_INBOUNDS = helpers.CLIENT_CONFIG_DIR / "inbounds.json"
SERVER_INBOUNDS = helpers.SERVER_CONFIG_DIR / "inbounds.json"
CLIENT_DIAG_PORT = "62023"
SERVER_DIAG_PORT = "62022"

SERVICE_TIMEOUT = 45.0
POLL_INTERVAL = 1.5


def _xp2p(host, *args: str, check: bool = False):
    result = openwrt_env.run_xp2p(host, *args)
    if check and result.rc != 0:
        pytest.fail(
            f"xp2p command failed (exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return result


def _wait_for_service_state(host, role: str, expected_active: bool) -> None:
    deadline = time.time() + SERVICE_TIMEOUT
    script = f"/etc/init.d/xp2p-{role}"
    last = None
    while time.time() < deadline:
        result = host.run(f"{script} running")
        active = result.rc == 0
        if active == expected_active:
            return
        last = result
        time.sleep(POLL_INTERVAL)
    stdout = getattr(last, "stdout", "") or ""
    stderr = getattr(last, "stderr", "") or ""
    state = "active" if expected_active else "inactive"
    raise AssertionError(
        f"xp2p {role} service did not reach {state} state.\nSTDOUT:\n{stdout}\nSTDERR:\n{stderr}"
    )


def _clear_logs(host, *paths: PurePosixPath) -> None:
    for path in paths:
        helpers.remove_path(host, path)


def _wait_for_log_entry(host, path: PurePosixPath, phrase: str, timeout: float = 60.0) -> None:
    deadline = time.time() + timeout
    last = ""
    target = phrase.lower()
    while time.time() < deadline:
        if helpers.path_exists(host, path):
            content = helpers.read_text(host, path)
            last = content
            if target in content.lower():
                return
        time.sleep(POLL_INTERVAL)
    raise AssertionError(f"{path} missing phrase {phrase!r}. Last content:\n{last}")


def _wait_for_diag_listener(host, port: str) -> None:
    deadline = time.time() + SERVICE_TIMEOUT
    while time.time() < deadline:
        result = host.run(f"netstat -ltn 2>/dev/null | grep ':{port} '")
        if result.rc == 0:
            return
        time.sleep(POLL_INTERVAL)
    pytest.fail(f"Diagnostics listener on port {port} not found after {SERVICE_TIMEOUT} seconds.")


def _stop_service(role: str, runner) -> None:
    runner(role, "service", "stop")


@pytest.mark.host
@pytest.mark.linux
def test_openwrt_client_service_cli_controls_procd(openwrt_host, xp2p_openwrt_ipk):
    openwrt_env.install_ipk_on_host(openwrt_host, xp2p_openwrt_ipk, force=True)
    runner = lambda *cmd, check=False: _xp2p(openwrt_host, *cmd, check=check)
    helpers.cleanup_client_install(openwrt_host, runner)
    try:
        runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.50.10",
            "--user",
            "svc-client@example.com",
            "--password",
            "svc-client-secret",
            "--force",
            check=True,
        )

        _clear_logs(openwrt_host, CLIENT_SERVICE_LOG, CLIENT_XRAY_LOG)
        _stop_service("client", runner)
        _wait_for_service_state(openwrt_host, "client", expected_active=False)

        runner("client", "service", "start", check=True)
        _wait_for_service_state(openwrt_host, "client", expected_active=True)
        status = runner("client", "service", "status")
        assert status.rc == 0, f"xp2p client service status failed:\n{status.stdout}\n{status.stderr}"
        _wait_for_diag_listener(openwrt_host, CLIENT_DIAG_PORT)

        runner("client", "service", "stop", check=True)
        _wait_for_service_state(openwrt_host, "client", expected_active=False)
        status_after = runner("client", "service", "status")
        assert (
            status_after.rc == 3
        ), f"Expected xp2p client service status rc=3 when inactive, got {status_after.rc}"
    finally:
        runner("client", "service", "stop")
        helpers.cleanup_client_install(openwrt_host, runner)
        _clear_logs(openwrt_host, CLIENT_SERVICE_LOG, CLIENT_XRAY_LOG)


@pytest.mark.host
@pytest.mark.linux
@pytest.mark.parametrize("role", ["client", "server"])
def test_openwrt_service_restarts_when_config_changes(openwrt_host, xp2p_openwrt_ipk, role):
    openwrt_env.install_ipk_on_host(openwrt_host, xp2p_openwrt_ipk, force=True)
    runner = lambda *cmd, check=False: _xp2p(openwrt_host, *cmd, check=check)
    if role == "client":
        helpers.cleanup_client_install(openwrt_host, runner)
        helpers.cleanup_server_install(openwrt_host, runner)
        service_log = CLIENT_SERVICE_LOG
        xray_log = CLIENT_XRAY_LOG
        diag_port = CLIENT_DIAG_PORT

        def install_fn():
            runner(
                "client",
                "install",
                "--path",
                helpers.INSTALL_ROOT.as_posix(),
                "--config-dir",
                helpers.CLIENT_CONFIG_DIR_NAME,
                "--host",
                "10.55.55.60",
                "--user",
                "svc-change@example.com",
                "--password",
                "svc-change-secret",
                "--force",
                check=True,
            )

        def change_fn():
            runner(
                "client",
                "redirect",
                "add",
                "--path",
                helpers.INSTALL_ROOT.as_posix(),
                "--config-dir",
                helpers.CLIENT_CONFIG_DIR_NAME,
                "--cidr",
                "198.18.0.0/15",
                "--host",
                "10.55.55.60",
                check=True,
            )

        def revert_fn():
            runner(
                "client",
                "redirect",
                "remove",
                "--path",
                helpers.INSTALL_ROOT.as_posix(),
                "--config-dir",
                helpers.CLIENT_CONFIG_DIR_NAME,
                "--cidr",
                "198.18.0.0/15",
                "--host",
                "10.55.55.60",
                check=True,
            )

    else:
        helpers.cleanup_server_install(openwrt_host, runner)
        service_log = SERVER_SERVICE_LOG
        xray_log = SERVER_XRAY_LOG
        diag_port = SERVER_DIAG_PORT

        def install_fn():
            runner(
                "server",
                "install",
                "--path",
                helpers.INSTALL_ROOT.as_posix(),
                "--config-dir",
                helpers.SERVER_CONFIG_DIR_NAME,
                "--port",
                "62145",
                "--host",
                "svc-server.example.com",
                "--force",
                check=True,
            )

        user_id = "svc-restart@example.com"

        def change_fn():
            runner(
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
            runner(
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

    install_fn()
    try:
        _clear_logs(openwrt_host, service_log, xray_log)
        runner(role, "service", "stop")
        runner(role, "service", "start", check=True)
        _wait_for_service_state(openwrt_host, role, expected_active=True)
        _wait_for_diag_listener(openwrt_host, diag_port)

        change_applied = False
        try:
            change_fn()
            change_applied = True
        finally:
            if change_applied:
                revert_fn()

        _wait_for_log_entry(openwrt_host, service_log, "service configuration change detected")
        _wait_for_service_state(openwrt_host, role, expected_active=True)
    finally:
        runner(role, "service", "stop")
        if role == "client":
            helpers.cleanup_client_install(openwrt_host, runner)
        else:
            helpers.cleanup_server_install(openwrt_host, runner)
        _clear_logs(openwrt_host, service_log, xray_log)


@pytest.mark.host
@pytest.mark.linux
@pytest.mark.parametrize("role", ["client", "server"])
def test_openwrt_service_stops_after_invalid_config(openwrt_host, xp2p_openwrt_ipk, role):
    openwrt_env.install_ipk_on_host(openwrt_host, xp2p_openwrt_ipk, force=True)
    runner = lambda *cmd, check=False: _xp2p(openwrt_host, *cmd, check=check)
    if role == "client":
        helpers.cleanup_client_install(openwrt_host, runner)
        service_log = CLIENT_SERVICE_LOG
        xray_log = CLIENT_XRAY_LOG
        config_path = CLIENT_INBOUNDS

        runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.70.70",
            "--user",
            "svc-fail@example.com",
            "--password",
            "svc-fail-secret",
            "--force",
            check=True,
        )
    else:
        helpers.cleanup_server_install(openwrt_host, runner)
        service_log = SERVER_SERVICE_LOG
        xray_log = SERVER_XRAY_LOG
        config_path = SERVER_INBOUNDS

        runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--port",
            "62165",
            "--host",
            "svc-server.example.com",
            "--force",
            check=True,
        )

    try:
        _clear_logs(openwrt_host, service_log, xray_log)
        runner(role, "service", "stop")
        helpers.write_text(openwrt_host, config_path, "BROKEN-CONFIG")
        runner(role, "service", "start")

        _wait_for_log_entry(openwrt_host, service_log, "exceeded restart limit")
        _wait_for_service_state(openwrt_host, role, expected_active=False)
        log_content = helpers.read_text(openwrt_host, service_log)
        assert "attempt" in log_content.lower()
        assert helpers.path_exists(openwrt_host, xray_log), f"{role} xray log missing"
        assert helpers.read_text(openwrt_host, xray_log).strip(), f"{role} xray log is empty"
    finally:
        runner(role, "service", "stop")
        if role == "client":
            helpers.cleanup_client_install(openwrt_host, runner)
        else:
            helpers.cleanup_server_install(openwrt_host, runner)
        _clear_logs(openwrt_host, service_log, xray_log)


@pytest.mark.host
@pytest.mark.linux
def test_openwrt_opkg_removal_and_purge_cleanup(openwrt_host, xp2p_openwrt_ipk):
    openwrt_env.install_ipk_on_host(openwrt_host, xp2p_openwrt_ipk, force=True)
    runner = lambda *cmd, check=False: _xp2p(openwrt_host, *cmd, check=check)
    helpers.cleanup_client_install(openwrt_host, runner)
    helpers.cleanup_server_install(openwrt_host, runner)

    def _install_roles():
        runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.90.10",
            "--user",
            "svc-clean@example.com",
            "--password",
            "svc-clean-secret",
            "--force",
            check=True,
        )
        runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--port",
            "62190",
            "--host",
            "svc-clean.example.com",
            "--force",
            check=True,
        )

    _install_roles()
    try:
        remove_result = openwrt_host.run("opkg remove xp2p")
        if remove_result.rc != 0:
            pytest.fail(
                f"opkg remove xp2p failed (exit {remove_result.rc}).\nSTDOUT:\n{remove_result.stdout}\nSTDERR:\n{remove_result.stderr}"
            )
        assert not helpers.path_exists(openwrt_host, PurePosixPath("/usr/bin/xp2p"))
        assert helpers.path_exists(openwrt_host, helpers.INSTALL_ROOT), "/etc/xp2p should persist after remove"
        assert helpers.path_exists(openwrt_host, helpers.LOG_ROOT), "/var/log/xp2p should persist after remove"

        # Reinstall to test purge scenario
        openwrt_env.install_ipk_on_host(openwrt_host, xp2p_openwrt_ipk, force=True)
        _install_roles()

        purge_result = openwrt_host.run("opkg remove --force-removal-of-dependent-packages xp2p")
        if purge_result.rc != 0:
            pytest.fail(
                f"opkg purge xp2p failed (exit {purge_result.rc}).\nSTDOUT:\n{purge_result.stdout}\nSTDERR:\n{purge_result.stderr}"
            )
        assert not helpers.path_exists(openwrt_host, helpers.INSTALL_ROOT), "/etc/xp2p should be removed after purge"
        assert not helpers.path_exists(openwrt_host, helpers.LOG_ROOT), "/var/log/xp2p should be removed after purge"
        assert not helpers.path_exists(openwrt_host, PurePosixPath("/etc/init.d/xp2p-client"))
        assert not helpers.path_exists(openwrt_host, PurePosixPath("/etc/init.d/xp2p-server"))
    finally:
        openwrt_env.install_ipk_on_host(openwrt_host, xp2p_openwrt_ipk, force=True)
        helpers.cleanup_client_install(openwrt_host, runner)
        helpers.cleanup_server_install(openwrt_host, runner)
        _clear_logs(openwrt_host, CLIENT_SERVICE_LOG, CLIENT_XRAY_LOG, SERVER_SERVICE_LOG, SERVER_XRAY_LOG)
