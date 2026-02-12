from __future__ import annotations

import time
from pathlib import PurePosixPath

import pytest
from testinfra.host import Host

from tests.host.linux import _helpers as helpers
from tests.host.linux import env as linux_env

pytestmark = [pytest.mark.host, pytest.mark.linux]

CLIENT_DEPLOY_LOG = PurePosixPath("/tmp/xp2p-client-deploy.log")
SERVER_DEPLOY_LOG = PurePosixPath("/tmp/xp2p-server-deploy.log")
DEPLOY_PORT = "62125"
TROJAN_PORT = "58601"
LOG_WAIT_TIMEOUT = 30
CLIENT_TUN = "xp2pc"
SERVER_TUN = "xp2ps"
CLIENT_TUN_ADDR = "198.18.0.1/30"
SERVER_TUN_ADDR = "198.18.0.5/30"
CLIENT_TUN_CIDR = "198.18.0.0/30"
SERVER_TUN_CIDR = "198.18.0.4/30"
SERVER_DEPLOY_DIAG_PORT = "62032"
SERVER_DIAG_PORT = "62022"
CLIENT_DIAG_PORT = "62023"


@pytest.mark.host
@pytest.mark.linux
def test_client_deploy_end_to_end(client_host, server_host, xp2p_client_runner, xp2p_server_runner):
    helpers.cleanup_client_install(client_host, xp2p_client_runner)
    helpers.cleanup_server_install(server_host, xp2p_server_runner)
    client_ip = helpers.detect_primary_ipv4(client_host)
    server_ip = _detect_host_ipv4(server_host)
    trojan_user = "deploy-suite@example.com"
    trojan_password = "deploy-pass-123"

    for host, log_path in (
        (client_host, CLIENT_DEPLOY_LOG),
        (server_host, SERVER_DEPLOY_LOG),
    ):
        helpers.remove_path(host, log_path)
        helpers.remove_path(host, helpers.HEARTBEAT_STATE_FILE)
        linux_env.run_guest_script(host, "scripts/linux/kill_xp2p_processes.sh")

    client_pid = None
    server_pid = None
    try:
        client_pid = _start_client_deploy(
            client_host,
            log_path=CLIENT_DEPLOY_LOG,
            remote_host=server_ip,
            deploy_port=DEPLOY_PORT,
            trojan_user=trojan_user,
            trojan_password=trojan_password,
            trojan_port=TROJAN_PORT,
        )
        link = _wait_for_client_link(client_host, CLIENT_DEPLOY_LOG)
        assert link.startswith("trojan://"), "xp2p client deploy did not emit trojan link"

        server_pid = _start_server_deploy(
            server_host,
            log_path=SERVER_DEPLOY_LOG,
            listen_addr=f":{DEPLOY_PORT}",
            deploy_link=link,
        )

        _wait_for_log_phrase(
            server_host,
            SERVER_DEPLOY_LOG,
            "server deploy: manifest decrypted",
            timeout=LOG_WAIT_TIMEOUT,
        )
        _wait_for_log_phrase(
            server_host,
            SERVER_DEPLOY_LOG,
            "server deploy: starting xray-core",
            timeout=LOG_WAIT_TIMEOUT,
        )
        _wait_for_log_phrase(
            client_host,
            CLIENT_DEPLOY_LOG,
            "client deploy: trojan link received",
            timeout=LOG_WAIT_TIMEOUT,
        )
        _wait_for_log_phrase(
            client_host,
            CLIENT_DEPLOY_LOG,
            "client deploy: local install completed",
            timeout=LOG_WAIT_TIMEOUT,
        )
        _wait_for_log_phrase(
            client_host,
            CLIENT_DEPLOY_LOG,
            "client deploy: ping ok",
            timeout=LOG_WAIT_TIMEOUT,
        )
        _wait_for_log_phrase(
            client_host,
            CLIENT_DEPLOY_LOG,
            "client deploy: client run active",
            timeout=LOG_WAIT_TIMEOUT,
        )

        _assert_client_install_artifacts(client_host, server_ip, trojan_user, trojan_password)
        _assert_client_state(client_host, server_ip)
        _assert_client_routing(client_host, server_ip)

        heartbeat_state = helpers.wait_for_heartbeat_state(
            client_host,
            timeout_seconds=LOG_WAIT_TIMEOUT,
        )
        helpers.assert_heartbeat_entry(
            heartbeat_state,
            helpers.expected_proxy_tag(server_ip),
            host=server_ip,
            user=trojan_user,
            client_ip=client_ip,
        )
    finally:
        if client_pid:
            linux_env.run_guest_script(client_host, "scripts/linux/stop_process.sh", str(client_pid))
        if server_pid:
            linux_env.run_guest_script(server_host, "scripts/linux/stop_process.sh", str(server_pid))
        for host in (client_host, server_host):
            linux_env.run_guest_script(host, "scripts/linux/kill_xp2p_processes.sh")
        helpers.cleanup_client_install(client_host, xp2p_client_runner)
        helpers.cleanup_server_install(server_host, xp2p_server_runner)
        for host, log_path in (
            (client_host, CLIENT_DEPLOY_LOG),
            (server_host, SERVER_DEPLOY_LOG),
        ):
            helpers.remove_path(host, log_path)
            helpers.remove_path(host, helpers.HEARTBEAT_STATE_FILE)


@pytest.mark.host
@pytest.mark.linux
def test_server_deploy_falls_back_to_self_signed_on_invalid_cert(
    client_host,
    server_host,
    xp2p_client_runner,
    xp2p_server_runner,
):
    helpers.cleanup_client_install(client_host, xp2p_client_runner)
    helpers.cleanup_server_install(server_host, xp2p_server_runner)
    server_ip = _detect_host_ipv4(server_host)
    trojan_user = "deploy-invalid-cert@example.com"
    trojan_password = "deploy-invalid-cert-pass"
    bad_cert = PurePosixPath("/tmp/xp2p-invalid-cert.pem")
    bad_key = PurePosixPath("/tmp/xp2p-invalid-key.pem")

    for host, log_path in (
        (client_host, CLIENT_DEPLOY_LOG),
        (server_host, SERVER_DEPLOY_LOG),
    ):
        helpers.remove_path(host, log_path)
        helpers.remove_path(host, helpers.HEARTBEAT_STATE_FILE)
        linux_env.run_guest_script(host, "scripts/linux/kill_xp2p_processes.sh")
        linux_env.run_guest_script(host, "scripts/linux/remove_path.sh", bad_cert.as_posix())
        linux_env.run_guest_script(host, "scripts/linux/remove_path.sh", bad_key.as_posix())

    client_pid = None
    server_pid = None
    try:
        client_pid = _start_client_deploy(
            client_host,
            log_path=CLIENT_DEPLOY_LOG,
            remote_host=server_ip,
            deploy_port=DEPLOY_PORT,
            trojan_user=trojan_user,
            trojan_password=trojan_password,
            trojan_port=TROJAN_PORT,
        )
        link = _wait_for_client_link(client_host, CLIENT_DEPLOY_LOG)

        server_pid = _start_server_deploy_with_args(
            server_host,
            log_path=SERVER_DEPLOY_LOG,
            listen_addr=f":{DEPLOY_PORT}",
            deploy_link=link,
            extra_args=[
                "--server-cert",
                bad_cert.as_posix(),
                "--server-key",
                bad_key.as_posix(),
            ],
        )

        _wait_for_log_phrase(
            server_host,
            SERVER_DEPLOY_LOG,
            "server deploy: manifest decrypted",
            timeout=LOG_WAIT_TIMEOUT,
        )
        _wait_for_log_phrase(
            server_host,
            SERVER_DEPLOY_LOG,
            "server deploy: certificate validation failed, using self-signed",
            timeout=LOG_WAIT_TIMEOUT,
        )
        _wait_for_log_phrase(
            server_host,
            SERVER_DEPLOY_LOG,
            "server deploy: starting xray-core",
            timeout=LOG_WAIT_TIMEOUT,
        )
        _wait_for_log_phrase(
            client_host,
            CLIENT_DEPLOY_LOG,
            "client deploy: local install completed",
            timeout=LOG_WAIT_TIMEOUT,
        )

        cert_path = helpers.SERVER_CONFIG_DIR / "cert.pem"
        key_path = helpers.SERVER_CONFIG_DIR / "key.pem"
        assert helpers.path_exists(server_host, cert_path), f"Expected cert at {cert_path}"
        assert helpers.path_exists(server_host, key_path), f"Expected key at {key_path}"

        inbounds = helpers.read_json(server_host, helpers.SERVER_CONFIG_DIR / "inbounds.json")
        trojan = _find_trojan_inbound(inbounds)
        tls_settings = trojan.get("streamSettings", {}).get("tlsSettings", {})
        assert tls_settings.get("allowInsecure") is True
        certificates = tls_settings.get("certificates", [])
        assert certificates, "Expected TLS certificates after deploy fallback"
        primary = certificates[0]
        assert primary.get("certificateFile") == cert_path.as_posix()
        assert primary.get("keyFile") == key_path.as_posix()
    finally:
        if client_pid:
            linux_env.run_guest_script(client_host, "scripts/linux/stop_process.sh", str(client_pid))
        if server_pid:
            linux_env.run_guest_script(server_host, "scripts/linux/stop_process.sh", str(server_pid))
        for host in (client_host, server_host):
            linux_env.run_guest_script(host, "scripts/linux/kill_xp2p_processes.sh")
        helpers.cleanup_client_install(client_host, xp2p_client_runner)
        helpers.cleanup_server_install(server_host, xp2p_server_runner)
        for host, log_path in (
            (client_host, CLIENT_DEPLOY_LOG),
            (server_host, SERVER_DEPLOY_LOG),
        ):
            helpers.remove_path(host, log_path)
            helpers.remove_path(host, helpers.HEARTBEAT_STATE_FILE)


@pytest.mark.host
@pytest.mark.linux
def test_deploy_tun_with_multiple_reverse_redirects(
    client_host,
    server_host,
    xp2p_client_runner,
    xp2p_server_runner,
):
    helpers.cleanup_client_install(client_host, xp2p_client_runner)
    helpers.cleanup_server_install(server_host, xp2p_server_runner)
    server_ip = _detect_host_ipv4(server_host)
    client_ip = helpers.detect_primary_ipv4(client_host)
    user_one = "deploy-tun-one@example.com"
    pass_one = "deploy-tun-pass-1"
    user_two = "deploy-tun-two@example.com"
    pass_two = "deploy-tun-pass-2"

    def cleanup_logs():
        xp2p_client_runner("client", "service", "stop")
        xp2p_server_runner("server", "service", "stop")
        for host, log_path, heartbeat_path in (
            (client_host, CLIENT_DEPLOY_LOG, helpers.CLIENT_HEARTBEAT_STATE_FILE),
            (server_host, SERVER_DEPLOY_LOG, helpers.SERVER_HEARTBEAT_STATE_FILE),
        ):
            helpers.remove_path(host, log_path)
            helpers.remove_path(host, heartbeat_path)
            linux_env.run_guest_script(host, "scripts/linux/kill_xp2p_processes.sh")
            host.run(
                "sudo -n fuser -k 62022/tcp 62022/udp 62023/tcp 62023/udp "
                "62032/tcp 62032/udp >/dev/null 2>&1 || true"
            )

    def wait_for_tun_ready():
        result = linux_env.run_guest_script(
            client_host,
            "scripts/linux/assert_tun_addr.sh",
            CLIENT_TUN,
            CLIENT_TUN_ADDR,
            "20",
        )
        if result.rc != 0:
            pytest.fail(
                "Client TUN not ready.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        result = linux_env.run_guest_script(
            server_host,
            "scripts/linux/assert_tun_addr.sh",
            SERVER_TUN,
            SERVER_TUN_ADDR,
            "20",
        )
        if result.rc != 0:
            pytest.fail(
                "Server TUN not ready.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )

    def _wait_for_port(host: Host, port: str, *, timeout: float = 20.0) -> None:
        deadline = time.time() + timeout
        while time.time() < deadline:
            result = host.run(f"sudo -n ss -lnt | grep -q ':{port} '")
            if result.rc == 0:
                return
            time.sleep(1.0)
        pytest.fail(f"Port {port} did not open within {timeout}s")

    def _wait_for_route(host: Host, cidr: str, dev: str, *, timeout: float = 20.0) -> None:
        deadline = time.time() + timeout
        while time.time() < deadline:
            result = host.run(f"ip route show {cidr} | grep -q 'dev {dev}'")
            if result.rc == 0:
                return
            time.sleep(1.0)
        routes = host.run("ip route").stdout or ""
        pytest.fail(f"Route {cidr} via {dev} not found.\nRoutes:\n{routes}")

    def assert_ping_zero_loss(
        runner,
        target: str,
        port: str,
        label: str,
        *,
        debug_hosts: list[Host] | None = None,
    ) -> None:
        last_result = None
        for _ in range(3):
            last_result = runner(
                "ping",
                target,
                "--port",
                port,
                "--count",
                "3",
                check=False,
            )
            stdout = (last_result.stdout or "").lower()
            if "0% loss" in stdout:
                return
            time.sleep(2.0)
        stdout = last_result.stdout if last_result else ""
        stderr = last_result.stderr if last_result else ""
        debug = ""
        for host in debug_hosts or []:
            routes = host.run("ip route").stdout or ""
            addrs = host.run("ip addr").stdout or ""
            sockets = host.run("sudo -n ss -lnt").stdout or ""
            debug += (
                f"\nhost={host.backend.hostname}\n"
                f"routes:\n{routes}\n"
                f"addr:\n{addrs}\n"
                f"sockets:\n{sockets}\n"
            )
        raise AssertionError(
            f"xp2p ping {label} did not report zero loss.\n"
            f"STDOUT:\n{stdout}\nSTDERR:\n{stderr}\n{debug}"
        )

    def _restart_services() -> None:
        xp2p_client_runner("client", "service", "stop")
        xp2p_server_runner("server", "service", "stop")
        for host in (client_host, server_host):
            linux_env.run_guest_script(host, "scripts/linux/kill_xp2p_processes.sh")
        server_host.run(
            "sudo -n fuser -k 62022/tcp 62022/udp 62023/tcp 62023/udp "
            "62032/tcp 62032/udp >/dev/null 2>&1 || true"
        )
        xp2p_client_runner("client", "service", "start", check=True)
        xp2p_server_runner("server", "service", "start", check=True)

    client_pid = None
    server_pid = None
    try:
        cleanup_logs()
        xp2p_server_runner("server", "service", "stop")

        client_pid = _start_client_deploy(
            client_host,
            log_path=CLIENT_DEPLOY_LOG,
            remote_host=server_ip,
            deploy_port=DEPLOY_PORT,
            trojan_user=user_one,
            trojan_password=pass_one,
            trojan_port=TROJAN_PORT,
        )
        link = _wait_for_client_link(client_host, CLIENT_DEPLOY_LOG)
        server_pid = _start_server_deploy(
            server_host,
            log_path=SERVER_DEPLOY_LOG,
            listen_addr=f":{DEPLOY_PORT}",
            deploy_link=link,
            global_args=["--diag-service-port", SERVER_DEPLOY_DIAG_PORT],
        )

        _wait_for_log_phrase(
            client_host,
            CLIENT_DEPLOY_LOG,
            "client deploy: local install completed",
            timeout=LOG_WAIT_TIMEOUT,
        )
        _wait_for_log_phrase(
            server_host,
            SERVER_DEPLOY_LOG,
            "server deploy: server service started",
            timeout=LOG_WAIT_TIMEOUT,
        )
        if client_pid:
            linux_env.run_guest_script(client_host, "scripts/linux/stop_process.sh", str(client_pid))
            client_pid = None
        if server_pid:
            linux_env.run_guest_script(server_host, "scripts/linux/stop_process.sh", str(server_pid))
            server_pid = None
        xp2p_client_runner("client", "service", "stop")
        xp2p_server_runner("server", "service", "stop")
        for host in (client_host, server_host):
            linux_env.run_guest_script(host, "scripts/linux/kill_xp2p_processes.sh")
        server_host.run(
            "sudo -n fuser -k 62022/tcp 62022/udp 62023/tcp 62023/udp "
            "62032/tcp 62032/udp >/dev/null 2>&1 || true"
        )
        xp2p_client_runner(
            "client",
            "mode",
            "tun",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            check=True,
        )
        xp2p_server_runner(
            "server",
            "mode",
            "tun",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            check=True,
        )
        _restart_services()
        wait_for_tun_ready()

        reverse_one = helpers.expected_reverse_tag(user_one, server_ip)
        xp2p_client_runner(
            "client",
            "redirect",
            "add",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--cidr",
            SERVER_TUN_CIDR,
            "--host",
            server_ip,
            check=True,
        )
        xp2p_server_runner(
            "server",
            "redirect",
            "add",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--cidr",
            CLIENT_TUN_CIDR,
            "--tag",
            reverse_one,
            "--host",
            server_ip,
            check=True,
        )
        _restart_services()
        _wait_for_port(server_host, SERVER_DIAG_PORT)
        _wait_for_port(client_host, CLIENT_DIAG_PORT)
        _wait_for_route(client_host, SERVER_TUN_CIDR, CLIENT_TUN)
        _wait_for_route(server_host, CLIENT_TUN_CIDR, SERVER_TUN)

        assert_ping_zero_loss(
            xp2p_client_runner,
            SERVER_TUN_ADDR.split("/")[0],
            SERVER_DIAG_PORT,
            "client->server",
            debug_hosts=[client_host, server_host],
        )
        assert_ping_zero_loss(
            xp2p_server_runner,
            CLIENT_TUN_ADDR.split("/")[0],
            CLIENT_DIAG_PORT,
            "server->client",
            debug_hosts=[client_host, server_host],
        )

        helpers.cleanup_client_install(client_host, xp2p_client_runner)
        linux_env.run_guest_script(client_host, "scripts/linux/kill_xp2p_processes.sh")

        cleanup_logs()
        xp2p_server_runner("server", "service", "stop")

        client_pid = _start_client_deploy(
            client_host,
            log_path=CLIENT_DEPLOY_LOG,
            remote_host=server_ip,
            deploy_port=DEPLOY_PORT,
            trojan_user=user_two,
            trojan_password=pass_two,
            trojan_port=TROJAN_PORT,
        )
        link = _wait_for_client_link(client_host, CLIENT_DEPLOY_LOG)
        server_pid = _start_server_deploy(
            server_host,
            log_path=SERVER_DEPLOY_LOG,
            listen_addr=f":{DEPLOY_PORT}",
            deploy_link=link,
            global_args=["--diag-service-port", SERVER_DEPLOY_DIAG_PORT],
        )

        _wait_for_log_phrase(
            client_host,
            CLIENT_DEPLOY_LOG,
            "client deploy: local install completed",
            timeout=LOG_WAIT_TIMEOUT,
        )
        _wait_for_log_phrase(
            server_host,
            SERVER_DEPLOY_LOG,
            "server deploy: server service started",
            timeout=LOG_WAIT_TIMEOUT,
        )
        if client_pid:
            linux_env.run_guest_script(client_host, "scripts/linux/stop_process.sh", str(client_pid))
            client_pid = None
        if server_pid:
            linux_env.run_guest_script(server_host, "scripts/linux/stop_process.sh", str(server_pid))
            server_pid = None
        xp2p_client_runner("client", "service", "stop")
        xp2p_server_runner("server", "service", "stop")
        for host in (client_host, server_host):
            linux_env.run_guest_script(host, "scripts/linux/kill_xp2p_processes.sh")
        server_host.run(
            "sudo -n fuser -k 62022/tcp 62022/udp 62023/tcp 62023/udp "
            "62032/tcp 62032/udp >/dev/null 2>&1 || true"
        )
        xp2p_client_runner(
            "client",
            "mode",
            "tun",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            check=True,
        )
        xp2p_server_runner(
            "server",
            "mode",
            "tun",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            check=True,
        )
        _restart_services()
        wait_for_tun_ready()

        reverse_two = helpers.expected_reverse_tag(user_two, server_ip)
        server_state = helpers.read_server_config(server_host)
        helpers.assert_server_reverse_state(server_state, reverse_one, user=user_one, host=server_ip)
        helpers.assert_server_reverse_state(server_state, reverse_two, user=user_two, host=server_ip)

        xp2p_server_runner(
            "server",
            "redirect",
            "add",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--cidr",
            CLIENT_TUN_CIDR,
            "--tag",
            reverse_two,
            "--host",
            server_ip,
            check=True,
        )
        xp2p_client_runner(
            "client",
            "redirect",
            "add",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--cidr",
            SERVER_TUN_CIDR,
            "--host",
            server_ip,
            check=True,
        )
        _restart_services()
        server_routing = helpers.read_json(server_host, helpers.SERVER_CONFIG_DIR / "routing.json")
        helpers.assert_server_redirect_rule(server_routing, CLIENT_TUN_CIDR, reverse_two)
        _wait_for_port(server_host, SERVER_DIAG_PORT)
        _wait_for_port(client_host, CLIENT_DIAG_PORT)
        _wait_for_route(client_host, SERVER_TUN_CIDR, CLIENT_TUN)
        _wait_for_route(server_host, CLIENT_TUN_CIDR, SERVER_TUN)

        assert_ping_zero_loss(
            xp2p_server_runner,
            CLIENT_TUN_ADDR.split("/")[0],
            CLIENT_DIAG_PORT,
            "server->client after second deploy",
            debug_hosts=[client_host, server_host],
        )
        assert_ping_zero_loss(
            xp2p_client_runner,
            SERVER_TUN_ADDR.split("/")[0],
            SERVER_DIAG_PORT,
            "client->server after second deploy",
            debug_hosts=[client_host, server_host],
        )
        try:
            heartbeat_state = helpers.wait_for_heartbeat_state(
                server_host,
                path=helpers.SERVER_HEARTBEAT_STATE_FILE,
                timeout_seconds=60,
            )
        except AssertionError as exc:
            service_log = helpers.LOG_ROOT / "server" / "service.log"
            xray_log = helpers.LOG_ROOT / "server" / "xray-service.log"
            log_details = ""
            for path in (service_log, xray_log):
                if helpers.path_exists(server_host, path):
                    tail = "\n".join((helpers.read_text(server_host, path) or "").splitlines()[-40:])
                    log_details += f"\n{path}:\n{tail}\n"
            raise AssertionError(
                "Server heartbeat state was not observed.\n"
                f"Service status:\n{xp2p_server_runner('server', 'service', 'status').stdout or ''}\n"
                f"{log_details}"
            ) from exc
        helpers.assert_heartbeat_entry(
            heartbeat_state,
            helpers.expected_proxy_tag(server_ip),
            host=server_ip,
            user=user_two,
            client_ip=client_ip,
        )
    finally:
        if client_pid:
            linux_env.run_guest_script(client_host, "scripts/linux/stop_process.sh", str(client_pid))
        if server_pid:
            linux_env.run_guest_script(server_host, "scripts/linux/stop_process.sh", str(server_pid))
        for host in (client_host, server_host):
            linux_env.run_guest_script(host, "scripts/linux/kill_xp2p_processes.sh")
        helpers.cleanup_client_install(client_host, xp2p_client_runner)
        helpers.cleanup_server_install(server_host, xp2p_server_runner)
        for host, log_path, heartbeat_path in (
            (client_host, CLIENT_DEPLOY_LOG, helpers.CLIENT_HEARTBEAT_STATE_FILE),
            (server_host, SERVER_DEPLOY_LOG, helpers.SERVER_HEARTBEAT_STATE_FILE),
        ):
            helpers.remove_path(host, log_path)
            helpers.remove_path(host, heartbeat_path)


def _start_client_deploy(
    host: Host,
    *,
    log_path: PurePosixPath,
    remote_host: str,
    deploy_port: str,
    trojan_user: str,
    trojan_password: str,
    trojan_port: str,
    diag_port: str | None = None,
) -> int:
    args = [
        "scripts/linux/start_xp2p_client_deploy.sh",
        log_path.as_posix(),
        remote_host,
        deploy_port,
        trojan_user,
        trojan_password,
        trojan_port,
    ]
    env = {}
    if diag_port:
        env["XP2P_GLOBAL_ARGS"] = f"--diag-service-port {diag_port}"
    if env:
        result = linux_env.run_guest_script_with_env(host, args[0], env, *args[1:])
    else:
        result = linux_env.run_guest_script(host, *args)
    if result.rc != 0:
        pytest.fail(
            "Failed to start xp2p client deploy.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    pid = _extract_marker(result.stdout, "__XP2P_PID__=")
    if not pid:
        pytest.fail(
            "xp2p client deploy script did not emit PID marker.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return int(pid)


def _start_server_deploy(
    host: Host,
    *,
    log_path: PurePosixPath,
    listen_addr: str,
    deploy_link: str,
    extra_args: list[str] | None = None,
    global_args: list[str] | None = None,
) -> int:
    return _start_server_deploy_with_args(
        host,
        log_path=log_path,
        listen_addr=listen_addr,
        deploy_link=deploy_link,
        extra_args=extra_args,
        global_args=global_args,
    )


def _start_server_deploy_with_args(
    host: Host,
    *,
    log_path: PurePosixPath,
    listen_addr: str,
    deploy_link: str,
    extra_args: list[str] | None = None,
    global_args: list[str] | None = None,
) -> int:
    args = [
        "scripts/linux/start_xp2p_server_deploy.sh",
        log_path.as_posix(),
        listen_addr,
        deploy_link,
    ]
    if extra_args:
        args.extend(extra_args)
    env = {}
    if global_args:
        env["XP2P_GLOBAL_ARGS"] = " ".join(global_args)
    if env:
        result = linux_env.run_guest_script_with_env(host, args[0], env, *args[1:])
    else:
        result = linux_env.run_guest_script(host, *args)
    if result.rc != 0:
        pytest.fail(
            "Failed to start xp2p server deploy.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    pid = _extract_marker(result.stdout, "__XP2P_PID__=")
    if not pid:
        pytest.fail(
            "xp2p server deploy script did not emit PID marker.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return int(pid)


def _assert_client_install_artifacts(host: Host, server_ip: str, user: str, password: str) -> None:
    assert helpers.path_exists(host, helpers.CLIENT_CONFIG_DIR), "client config directory missing after deploy"
    outbounds = helpers.read_json(host, helpers.CLIENT_CONFIG_DIR / "outbounds.json")
    helpers.assert_outbound(
        outbounds,
        server_ip,
        password,
        user,
        server_ip,
        allow_insecure=True,
    )


def _assert_client_state(host: Host, server_ip: str) -> None:
    state = helpers.read_client_config(host)
    recorded_hosts = {entry.get("hostname") for entry in state.get("endpoints", [])}
    assert recorded_hosts == {server_ip}, f"Unexpected endpoint entries recorded: {recorded_hosts}"


def _assert_client_routing(host: Host, server_ip: str) -> None:
    routing = helpers.read_json(host, helpers.CLIENT_CONFIG_DIR / "routing.json")
    helpers.assert_routing_rule(routing, server_ip)


def _find_trojan_inbound(data: dict) -> dict:
    for inbound in data.get("inbounds", []):
        if inbound.get("protocol") == "trojan":
            return inbound
    raise AssertionError("Expected trojan inbound in server configuration")


def _wait_for_client_link(host: Host, log_path: PurePosixPath) -> str:
    def _extract_link(text: str) -> str | None:
        for line in text.splitlines():
            if "client deploy: link generated" not in line:
                continue
            if "link:" not in line:
                continue
            return line.split("link:", 1)[1].strip()
        return None

    link = _wait_for_log_value(
        host,
        log_path,
        extractor=_extract_link,
        description="xp2p client deploy link",
        timeout=LOG_WAIT_TIMEOUT,
    )
    if not link:
        pytest.fail("xp2p client deploy log did not include a deploy link")
    return link


def _wait_for_log_phrase(host: Host, path: PurePosixPath, phrase: str, *, timeout: int) -> None:
    expected_variants = (phrase, f"xp2p: {phrase}")

    def _matcher(text: str) -> bool | None:
        for variant in expected_variants:
            if variant in text:
                return True
        return None

    _wait_for_log_value(
        host,
        path,
        extractor=_matcher,
        description=f"'{phrase}' in {path}",
        timeout=timeout,
    )


def _wait_for_log_value(
    host: Host,
    path: PurePosixPath,
    *,
    extractor,
    description: str,
    timeout: int,
):
    deadline = time.time() + timeout
    last_text = ""
    while time.time() < deadline:
        text = _read_optional_log(host, path)
        if text:
            value = extractor(text)
            if value:
                return value
            last_text = text
        time.sleep(1)
    tail = "\n".join((last_text or "").splitlines()[-30:])
    pytest.fail(f"Timed out waiting for {description}. Recent log tail:\n{tail}")


def _read_optional_log(host: Host, path: PurePosixPath) -> str:
    result = linux_env.run_guest_script(host, "scripts/linux/read_file.sh", path.as_posix())
    if result.rc == 0:
        return result.stdout or ""
    if result.rc == 3:
        return ""
    pytest.fail(
        f"Failed to read log {path} (exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
    )


def _extract_marker(output: str | None, marker: str) -> str | None:
    for raw in (output or "").splitlines():
        line = raw.strip()
        if line.startswith(marker):
            return line[len(marker) :].strip()
    return None


def _detect_host_ipv4(host: Host) -> str:
    result = linux_env.run_guest_script(host, "scripts/linux/get_primary_ipv4.sh")
    if result.rc != 0:
        pytest.fail(
            "Failed to detect IPv4 addresses.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    addresses = [line.strip() for line in (result.stdout or "").splitlines() if line.strip()]
    if not addresses:
        pytest.fail("No IPv4 addresses found on host")
    for addr in addresses:
        if not addr.startswith("10.0.2."):
            return addr
    return addresses[0]
