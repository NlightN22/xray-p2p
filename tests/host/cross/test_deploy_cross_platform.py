from __future__ import annotations

from collections.abc import Iterator
from pathlib import Path, PurePosixPath

import pytest
from testinfra.host import Host

from tests.host.linux import _helpers as linux_helpers
from tests.host.linux import env as linux_env
from tests.host.win import env as win_env
from tests.host.cross import deploy_helpers as helpers

pytestmark = [pytest.mark.host, pytest.mark.cross]

DEPLOY_PORT = "62125"
TROJAN_PORT = "58601"
LOG_WAIT_TIMEOUT = 30

LINUX_ARTIFACT_ROOT = PurePosixPath("/srv/xray-p2p/build/artifacts/deploy")
WINDOWS_ARTIFACT_ROOT = Path(r"C:\xp2p\build\artifacts\deploy")

LINUX_CLIENT_LOG = LINUX_ARTIFACT_ROOT / "linux-client-deploy.log"
LINUX_SERVER_LOG = LINUX_ARTIFACT_ROOT / "linux-server-deploy.log"
WINDOWS_CLIENT_LOG = WINDOWS_ARTIFACT_ROOT / "windows-client-deploy.log"
WINDOWS_SERVER_LOG = WINDOWS_ARTIFACT_ROOT / "windows-server-deploy.log"

DEFAULT_WINDOWS_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
DEFAULT_LINUX_INSTALL_DIR = PurePosixPath("/etc/xp2p")
WINDOWS_HEARTBEAT_STATE_FILE = win_env.CONFIG_ROOT / "state-heartbeat.json"


@pytest.fixture(scope="session")
def linux_hosts() -> dict[str, Host]:
    linux_env.require_vagrant_environment()
    factory = linux_env.machine_host_factory()
    client = factory(linux_env.DEFAULT_CLIENT)
    server = factory(linux_env.DEFAULT_SERVER)
    linux_env.ensure_xp2p_installed(linux_env.DEFAULT_CLIENT, client)
    linux_env.ensure_xp2p_installed(linux_env.DEFAULT_SERVER, server)
    return {"client": client, "server": server}


@pytest.fixture(scope="session")
def windows_hosts() -> Iterator[dict[str, Host]]:
    win_env.require_vagrant_environment()
    server = win_env.get_ssh_host(win_env.DEFAULT_SERVER)
    client = win_env.get_ssh_host(win_env.DEFAULT_CLIENT)
    for host in (server, client):
        helpers.ensure_windows_ready(host)
    yield {"client": client, "server": server}
    msi_path = win_env.ensure_msi_package(server)
    win_env.uninstall_xp2p_from_msi(server, msi_path)
    win_env.uninstall_xp2p_from_msi(client, msi_path)


@pytest.mark.host
@pytest.mark.cross
def test_cross_deploy_linux_client_windows_server(linux_hosts, windows_hosts):
    client_host = linux_hosts["client"]
    server_host = windows_hosts["server"]
    linux_client_runner = helpers.linux_runner(client_host)
    win_server_runner = helpers.windows_runner(server_host)

    linux_helpers.cleanup_client_install(client_host, linux_client_runner)
    helpers.cleanup_windows_server_install(server_host, win_server_runner, DEFAULT_WINDOWS_INSTALL_DIR)
    linux_helpers.remove_path(client_host, linux_helpers.HEARTBEAT_STATE_FILE)
    win_env.remove_path(server_host, WINDOWS_HEARTBEAT_STATE_FILE)

    server_ip = helpers.detect_windows_ipv4(server_host)
    trojan_user = "deploy-cross@example.com"
    trojan_password = "deploy-cross-pass"

    def _run_scenario(install_dir: str | None, expect_error: bool) -> None:
        client_pid = None
        server_proc: helpers.WindowsProcInfo | None = None
        success = False
        helpers.reset_linux_logs(client_host, LINUX_CLIENT_LOG)
        helpers.reset_windows_logs(server_host, WINDOWS_SERVER_LOG)
        try:
            client_pid = helpers.start_linux_client_deploy(
                client_host,
                log_path=LINUX_CLIENT_LOG,
                remote_host=server_ip,
                deploy_port=DEPLOY_PORT,
                trojan_user=trojan_user,
                trojan_password=trojan_password,
                trojan_port=TROJAN_PORT,
                install_dir=install_dir,
            )
            link = helpers.wait_for_client_link_linux(client_host, LINUX_CLIENT_LOG, timeout=LOG_WAIT_TIMEOUT)
            helpers.assert_link_install_dir(link, install_dir)

            server_proc = helpers.start_windows_server_deploy(
                server_host,
                log_path=WINDOWS_SERVER_LOG,
                listen_addr=f":{DEPLOY_PORT}",
                deploy_link=link,
            )

            if expect_error:
                helpers.wait_for_error_phrase_linux(
                    client_host,
                    LINUX_CLIENT_LOG,
                    "server rejected deploy request",
                    timeout=LOG_WAIT_TIMEOUT,
                )
                helpers.wait_for_error_phrase_linux(
                    client_host,
                    LINUX_CLIENT_LOG,
                    "invalid install_dir for Windows",
                    timeout=LOG_WAIT_TIMEOUT,
                )
                success = True
                return

            helpers.wait_for_log_phrase_windows(
                server_host,
                server_proc,
                "server deploy: manifest decrypted",
                timeout=LOG_WAIT_TIMEOUT,
            )
            helpers.wait_for_log_phrase_windows(
                server_host,
                server_proc,
                "server deploy: starting xray-core",
                timeout=LOG_WAIT_TIMEOUT,
            )
            helpers.wait_for_log_phrase_linux(
                client_host,
                LINUX_CLIENT_LOG,
                "client deploy: trojan link received",
                timeout=LOG_WAIT_TIMEOUT,
            )
            helpers.wait_for_log_phrase_linux(
                client_host,
                LINUX_CLIENT_LOG,
                "client deploy: local install completed",
                timeout=LOG_WAIT_TIMEOUT,
            )
            helpers.wait_for_log_phrase_linux(
                client_host,
                LINUX_CLIENT_LOG,
                "client deploy: ping ok",
                timeout=LOG_WAIT_TIMEOUT,
            )
            helpers.wait_for_log_phrase_linux(
                client_host,
                LINUX_CLIENT_LOG,
                "client deploy: client run active",
                timeout=LOG_WAIT_TIMEOUT,
            )

            helpers.assert_windows_server_install_dir(server_host, DEFAULT_WINDOWS_INSTALL_DIR)
            success = True
        finally:
            if client_pid:
                linux_env.stop_process(client_host, str(client_pid))
            if server_proc:
                helpers.stop_windows_process(server_host, int(server_proc["pid"]))
            linux_helpers.cleanup_client_install(client_host, linux_client_runner)
            helpers.cleanup_windows_server_install(server_host, win_server_runner, DEFAULT_WINDOWS_INSTALL_DIR)
            linux_helpers.remove_path(client_host, linux_helpers.HEARTBEAT_STATE_FILE)
            win_env.remove_path(server_host, WINDOWS_HEARTBEAT_STATE_FILE)
            if success:
                helpers.reset_linux_logs(client_host, LINUX_CLIENT_LOG)
                helpers.reset_windows_logs(server_host, WINDOWS_SERVER_LOG)

    _run_scenario(install_dir=None, expect_error=False)
    _run_scenario(install_dir=str(DEFAULT_WINDOWS_INSTALL_DIR), expect_error=False)
    _run_scenario(install_dir="/invalid/path", expect_error=True)


@pytest.mark.host
@pytest.mark.cross
def test_cross_deploy_windows_client_linux_server(linux_hosts, windows_hosts):
    client_host = windows_hosts["client"]
    server_host = linux_hosts["server"]
    win_client_runner = helpers.windows_runner(client_host)
    linux_server_runner = helpers.linux_runner(server_host)

    helpers.cleanup_windows_client_install(client_host, win_client_runner, DEFAULT_WINDOWS_INSTALL_DIR)
    linux_helpers.cleanup_server_install(server_host, linux_server_runner)
    win_env.remove_path(client_host, WINDOWS_HEARTBEAT_STATE_FILE)
    linux_helpers.remove_path(server_host, linux_helpers.HEARTBEAT_STATE_FILE)

    server_ip = helpers.detect_linux_ipv4_non_nat(server_host)
    trojan_user = "deploy-cross@example.com"
    trojan_password = "deploy-cross-pass"

    def _run_scenario(install_dir: str | None) -> None:
        client_proc: helpers.WindowsProcInfo | None = None
        server_pid: int | None = None
        success = False
        helpers.reset_windows_logs(client_host, WINDOWS_CLIENT_LOG)
        helpers.reset_linux_logs(server_host, LINUX_SERVER_LOG)
        try:
            client_proc = helpers.start_windows_client_deploy(
                client_host,
                log_path=WINDOWS_CLIENT_LOG,
                remote_host=server_ip,
                deploy_port=DEPLOY_PORT,
                trojan_user=trojan_user,
                trojan_password=trojan_password,
                trojan_port=TROJAN_PORT,
                install_dir=install_dir,
            )
            link = helpers.wait_for_client_link_windows(client_host, client_proc, timeout=LOG_WAIT_TIMEOUT)
            helpers.assert_link_install_dir(link, install_dir)

            server_pid = helpers.start_linux_server_deploy(
                server_host,
                log_path=LINUX_SERVER_LOG,
                listen_addr=f":{DEPLOY_PORT}",
                deploy_link=link,
            )
            helpers.wait_for_log_phrase_linux(
                server_host,
                LINUX_SERVER_LOG,
                "server deploy: starting listener",
                timeout=LOG_WAIT_TIMEOUT,
            )
            helpers.assert_windows_tcp_reachable(client_host, server_ip, int(DEPLOY_PORT))

            helpers.wait_for_log_phrase_linux(
                server_host,
                LINUX_SERVER_LOG,
                "server deploy: manifest decrypted",
                timeout=LOG_WAIT_TIMEOUT,
            )
            helpers.wait_for_log_phrase_linux(
                server_host,
                LINUX_SERVER_LOG,
                "server deploy: starting xray-core",
                timeout=LOG_WAIT_TIMEOUT,
            )
            helpers.wait_for_log_phrase_windows(
                client_host,
                client_proc,
                "client deploy: trojan link received",
                timeout=LOG_WAIT_TIMEOUT,
            )
            helpers.wait_for_log_phrase_windows(
                client_host,
                client_proc,
                "client deploy: local install completed",
                timeout=LOG_WAIT_TIMEOUT,
            )
            helpers.wait_for_log_phrase_windows(
                client_host,
                client_proc,
                "client deploy: ping ok",
                timeout=LOG_WAIT_TIMEOUT,
            )
            helpers.wait_for_log_phrase_windows(
                client_host,
                client_proc,
                "client deploy: client run active",
                timeout=LOG_WAIT_TIMEOUT,
            )

            helpers.assert_linux_server_install_dir(server_host, DEFAULT_LINUX_INSTALL_DIR)
            success = True
        finally:
            if client_proc:
                helpers.stop_windows_process(client_host, int(client_proc["pid"]))
            if server_pid:
                linux_env.stop_process(server_host, str(server_pid))
            helpers.cleanup_windows_client_install(client_host, win_client_runner, DEFAULT_WINDOWS_INSTALL_DIR)
            linux_helpers.cleanup_server_install(server_host, linux_server_runner)
            win_env.remove_path(client_host, WINDOWS_HEARTBEAT_STATE_FILE)
            linux_helpers.remove_path(server_host, linux_helpers.HEARTBEAT_STATE_FILE)
            if success:
                helpers.reset_windows_logs(client_host, WINDOWS_CLIENT_LOG)
                helpers.reset_linux_logs(server_host, LINUX_SERVER_LOG)

    _run_scenario(install_dir=None)
    _run_scenario(install_dir=str(DEFAULT_LINUX_INSTALL_DIR))
