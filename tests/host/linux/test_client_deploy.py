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
LOG_WAIT_TIMEOUT = 180


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


def _start_client_deploy(
    host: Host,
    *,
    log_path: PurePosixPath,
    remote_host: str,
    deploy_port: str,
    trojan_user: str,
    trojan_password: str,
    trojan_port: str,
) -> int:
    result = linux_env.run_guest_script(
        host,
        "scripts/linux/start_xp2p_client_deploy.sh",
        log_path.as_posix(),
        remote_host,
        deploy_port,
        trojan_user,
        trojan_password,
        trojan_port,
    )
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


def _start_server_deploy(host: Host, *, log_path: PurePosixPath, listen_addr: str, deploy_link: str) -> int:
    return _start_server_deploy_with_args(
        host,
        log_path=log_path,
        listen_addr=listen_addr,
        deploy_link=deploy_link,
    )


def _start_server_deploy_with_args(
    host: Host,
    *,
    log_path: PurePosixPath,
    listen_addr: str,
    deploy_link: str,
    extra_args: list[str] | None = None,
) -> int:
    args = [
        "scripts/linux/start_xp2p_server_deploy.sh",
        log_path.as_posix(),
        listen_addr,
        deploy_link,
    ]
    if extra_args:
        args.extend(extra_args)
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
    state = helpers.read_first_existing_json(host, helpers.CLIENT_STATE_FILES)
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
