from __future__ import annotations

import time

import pytest

from tests.host.linux import _helpers as helpers

CLIENT_SERVICE_LOG = helpers.LOG_ROOT / "client" / "service.log"
SERVER_SERVICE_LOG = helpers.LOG_ROOT / "server" / "service.log"

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


@pytest.mark.host
@pytest.mark.linux
def test_client_service_cli_controls_systemd(client_host, xp2p_client_runner):
    helpers.cleanup_client_install(client_host, xp2p_client_runner)
    try:
        xp2p_client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--host",
            "10.55.10.50",
            "--user",
            "svc-client@example.com",
            "--password",
            "svc-client-secret",
            check=True,
        )

        helpers.remove_path(client_host, CLIENT_SERVICE_LOG)
        xp2p_client_runner("client", "service", "stop")
        xp2p_client_runner("client", "service", "start", check=True)
        _wait_for_service_state(xp2p_client_runner, "client", expected_active=True)
        assert helpers.path_exists(client_host, CLIENT_SERVICE_LOG), "client service log was not created"

        xp2p_client_runner("client", "service", "stop", check=True)
        _wait_for_service_state(xp2p_client_runner, "client", expected_active=False)
    finally:
        xp2p_client_runner("client", "service", "stop")
        helpers.cleanup_client_install(client_host, xp2p_client_runner)
        helpers.remove_path(client_host, CLIENT_SERVICE_LOG)


@pytest.mark.host
@pytest.mark.linux
def test_server_service_cli_controls_systemd(server_host, xp2p_server_runner):
    helpers.cleanup_server_install(server_host, xp2p_server_runner)
    try:
        xp2p_server_runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--port",
            "62120",
            "--host",
            "svc-server.example.com",
            "--force",
            check=True,
        )

        helpers.remove_path(server_host, SERVER_SERVICE_LOG)
        xp2p_server_runner("server", "service", "stop")
        xp2p_server_runner("server", "service", "start", check=True)
        _wait_for_service_state(xp2p_server_runner, "server", expected_active=True)
        assert helpers.path_exists(server_host, SERVER_SERVICE_LOG), "server service log was not created"

        xp2p_server_runner("server", "service", "stop", check=True)
        _wait_for_service_state(xp2p_server_runner, "server", expected_active=False)
    finally:
        xp2p_server_runner("server", "service", "stop")
        helpers.cleanup_server_install(server_host, xp2p_server_runner)
        helpers.remove_path(server_host, SERVER_SERVICE_LOG)
