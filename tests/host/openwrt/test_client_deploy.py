from __future__ import annotations

import time
from pathlib import PurePosixPath
from urllib import parse

import pytest
from testinfra.host import Host

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env

pytestmark = [pytest.mark.host, pytest.mark.linux]

CLIENT_DEPLOY_LOG = PurePosixPath("/tmp/xp2p-client-deploy.log")
SERVER_DEPLOY_LOG = PurePosixPath("/tmp/xp2p-server-deploy.log")
DEPLOY_PORT = "62125"
TROJAN_PORT = "58601"
LOG_WAIT_TIMEOUT = 180


def _runner(host: Host):
    def _run(*args: str, check: bool = False):
        result = openwrt_env.run_xp2p(host, *args)
        if check and result.rc != 0:
            pytest.fail(
                "xp2p command failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        return result

    return _run


@pytest.mark.host
@pytest.mark.linux
def test_openwrt_client_deploy_end_to_end(openwrt_server_host, openwrt_client_host, xp2p_openwrt_ipk):
    server_runner = _runner(openwrt_server_host)
    client_runner = _runner(openwrt_client_host)

    openwrt_env.sync_build_output(openwrt_env.OPENWRT_MACHINES[0])
    openwrt_env.install_ipk_on_host(openwrt_server_host, xp2p_openwrt_ipk, force=True)
    openwrt_env.sync_build_output(openwrt_env.OPENWRT_MACHINES[1])
    openwrt_env.install_ipk_on_host(openwrt_client_host, xp2p_openwrt_ipk, force=True)

    helpers.cleanup_client_install(openwrt_client_host, client_runner)
    helpers.cleanup_server_install(openwrt_server_host, server_runner)

    client_ip = _detect_host_ipv4(openwrt_client_host)
    server_ip = _detect_host_ipv4(openwrt_server_host)
    trojan_user = "deploy-suite@example.com"
    trojan_password = "deploy-pass-123"

    for host, log_path in (
        (openwrt_client_host, CLIENT_DEPLOY_LOG),
        (openwrt_server_host, SERVER_DEPLOY_LOG),
    ):
        helpers.remove_path(host, log_path)
        helpers.remove_path(host, helpers.HEARTBEAT_STATE_FILE)
        host.run("pkill -f 'xp2p client run' >/dev/null 2>&1 || true")
        host.run("pkill -f 'xp2p server run' >/dev/null 2>&1 || true")
        host.run("pkill -f 'xp2p client deploy' >/dev/null 2>&1 || true")
        host.run("pkill -f 'xp2p server deploy' >/dev/null 2>&1 || true")
        host.run("pkill -f '/etc/xp2p/bin/xray' >/dev/null 2>&1 || true")

    client_pid = None
    server_pid = None
    try:
        client_pid = _start_client_deploy(
            openwrt_client_host,
            log_path=CLIENT_DEPLOY_LOG,
            remote_host=server_ip,
            deploy_port=DEPLOY_PORT,
            trojan_user=trojan_user,
            trojan_password=trojan_password,
            trojan_port=TROJAN_PORT,
        )
        link = _wait_for_client_link(openwrt_client_host, CLIENT_DEPLOY_LOG)
        assert link.startswith("trojan://"), "xp2p client deploy did not emit trojan link"

        server_pid = _start_server_deploy(
            openwrt_server_host,
            log_path=SERVER_DEPLOY_LOG,
            listen_addr=f":{DEPLOY_PORT}",
            deploy_link=link,
        )

        _wait_for_log_phrase(
            openwrt_server_host,
            SERVER_DEPLOY_LOG,
            "server deploy: manifest decrypted",
            timeout=LOG_WAIT_TIMEOUT,
        )
        _wait_for_log_phrase(
            openwrt_server_host,
            SERVER_DEPLOY_LOG,
            "server deploy: starting xray-core",
            timeout=LOG_WAIT_TIMEOUT,
        )
        _wait_for_log_phrase(
            openwrt_client_host,
            CLIENT_DEPLOY_LOG,
            "client deploy: trojan link received",
            timeout=LOG_WAIT_TIMEOUT,
        )
        _wait_for_log_phrase(
            openwrt_client_host,
            CLIENT_DEPLOY_LOG,
            "client deploy: local install completed",
            timeout=LOG_WAIT_TIMEOUT,
        )
        _wait_for_log_phrase(
            openwrt_client_host,
            CLIENT_DEPLOY_LOG,
            "client deploy: ping ok",
            timeout=LOG_WAIT_TIMEOUT,
        )
        _wait_for_log_phrase(
            openwrt_client_host,
            CLIENT_DEPLOY_LOG,
            "client deploy: client run active",
            timeout=LOG_WAIT_TIMEOUT,
        )

        _assert_link_matches(
            link,
            host=server_ip,
            trojan_port=TROJAN_PORT,
            trojan_user=trojan_user,
            trojan_password=trojan_password,
        )
        _assert_client_install_artifacts(openwrt_client_host, server_ip, trojan_user, trojan_password)
        _assert_client_state(openwrt_client_host, server_ip)
        _assert_client_routing(openwrt_client_host, server_ip)

        heartbeat_state = helpers.wait_for_heartbeat_state(
            openwrt_client_host,
            timeout_seconds=LOG_WAIT_TIMEOUT,
        )
        helpers.assert_heartbeat_entry(
            heartbeat_state,
            helpers.expected_proxy_tag(server_ip),
            host=server_ip,
            user=trojan_user,
        )
    finally:
        if client_pid:
            openwrt_client_host.run(f"kill {client_pid} >/dev/null 2>&1 || true")
        if server_pid:
            openwrt_server_host.run(f"kill {server_pid} >/dev/null 2>&1 || true")
        helpers.cleanup_client_install(openwrt_client_host, client_runner)
        helpers.cleanup_server_install(openwrt_server_host, server_runner)
        for host, log_path in (
            (openwrt_client_host, CLIENT_DEPLOY_LOG),
            (openwrt_server_host, SERVER_DEPLOY_LOG),
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
    result = openwrt_env.run_guest_script(
        host,
        "scripts/openwrt/start_xp2p_client_deploy.sh",
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
    result = openwrt_env.run_guest_script(
        host,
        "scripts/openwrt/start_xp2p_server_deploy.sh",
        log_path.as_posix(),
        listen_addr,
        deploy_link,
    )
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


def _assert_link_matches(
    link: str,
    *,
    host: str,
    trojan_port: str,
    trojan_user: str,
    trojan_password: str,
) -> None:
    parsed = parse.urlparse(link)
    assert parsed.scheme == "trojan", f"unexpected scheme in link: {parsed.scheme}"
    assert parsed.hostname == host, f"unexpected host in link (got {parsed.hostname}, want {host})"
    assert parsed.port == int(trojan_port), f"unexpected port in link (got {parsed.port}, want {trojan_port})"
    assert parsed.username == trojan_password, "trojan password is not in link userinfo"
    fragment = parse.unquote(parsed.fragment or "")
    assert fragment == trojan_user, f"trojan user fragment mismatch (got {fragment}, want {trojan_user})"
    query = parse.parse_qs(parsed.query)
    assert query.get("deploy_version") == ["2"], f"deploy_version missing/mismatch: {query}"
    assert query.get("install_dir") == [helpers.INSTALL_ROOT.as_posix()], f"install_dir mismatch: {query}"
    assert query.get("security") == ["tls"], f"security param missing: {query}"
    assert query.get("sni") == [host], f"sni mismatch (got {query.get('sni')}, want {host})"


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
    if not helpers.path_exists(host, path):
        return ""
    return helpers.read_text(host, path)


def _extract_marker(output: str | None, marker: str) -> str | None:
    for raw in (output or "").splitlines():
        line = raw.strip()
        if line.startswith(marker):
            return line[len(marker) :].strip()
    return None


def _detect_host_ipv4(host: Host) -> str:
    command = "ip -o -4 addr show scope global | awk '{print $4}' | cut -d/ -f1"
    result = host.run(command)
    if result.rc != 0:
        pytest.fail(
            "Failed to detect IPv4 addresses.\n"
            f"CMD: {command}\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    addresses = [line.strip() for line in (result.stdout or "").splitlines() if line.strip()]
    if not addresses:
        pytest.fail("No IPv4 addresses found on host")
    for addr in addresses:
        if not addr.startswith("10.0.2."):
            return addr
    return addresses[0]
