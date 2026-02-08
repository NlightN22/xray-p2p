from __future__ import annotations

from contextlib import contextmanager
import time

import pytest

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env
from tests.host.tunnel import common as tunnel_common

SERVER_MACHINE = openwrt_env.OPENWRT_MACHINES[0]
CLIENT_MACHINE = openwrt_env.OPENWRT_MACHINES[1]
SERVER_IP = "10.63.30.11"
CLIENT_IP = "10.63.30.12"
SERVER_DOMAIN = "edge.example.test"
SERVER_DIAGNOSTICS_PORT = 62022
CLIENT_DIAGNOSTICS_PORT = 62023
SERVER_FORWARD_PORT = 53341
CLIENT_FORWARD_PORT = 53331
pytestmark = [pytest.mark.host, pytest.mark.linux]
SERVER_HEARTBEAT_STATE_FILE = helpers.SERVER_HEARTBEAT_STATE_FILE
CLIENT_HEARTBEAT_STATE_FILE = helpers.CLIENT_HEARTBEAT_STATE_FILE


def _runner(host):
    def _run(*args: str, check: bool = False):
        result = openwrt_env.run_xp2p(host, *args)
        if check and result.rc != 0:
            pytest.fail(
                "xp2p command failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        return result

    return _run


def _update_hosts_entry(host, action: str, domain: str, ip: str | None = None) -> None:
    args = ["scripts/linux/update_hosts_entry_openwrt.sh", action]
    if action == "add":
        if not ip:
            raise AssertionError("IP is required for add action")
        args.extend([ip, domain])
    else:
        args.append(domain)
    result = openwrt_env.run_guest_script(host, *args)
    if result.rc != 0:
        pytest.fail(
            "guest script failed "
            f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


@pytest.fixture(scope="module")
def tunnel_environment(openwrt_host_factory, xp2p_openwrt_ipk):
    server_host = openwrt_host_factory(SERVER_MACHINE)
    client_host = openwrt_host_factory(CLIENT_MACHINE)
    server_runner = _runner(server_host)
    client_runner = _runner(client_host)
    server_install_path = helpers.INSTALL_ROOT.as_posix()
    client_primary_ip = helpers.detect_primary_ipv4(client_host)

    def cleanup():
        for host in (server_host, client_host):
            openwrt_env._stop_xp2p_services(host)
            host.run("pkill -f 'xp2p server run' >/dev/null 2>&1 || true")
            host.run("pkill -f 'xp2p client run' >/dev/null 2>&1 || true")
            host.run("pkill -f '/etc/xp2p/bin/xray' >/dev/null 2>&1 || true")
        helpers.cleanup_server_install(server_host, server_runner)
        helpers.cleanup_client_install(client_host, client_runner)
        helpers.remove_path(server_host, SERVER_HEARTBEAT_STATE_FILE)
        helpers.remove_path(client_host, CLIENT_HEARTBEAT_STATE_FILE)
        for host in (server_host, client_host):
            _update_hosts_entry(host, "remove", SERVER_DOMAIN)

    cleanup()
    for machine, host in ((SERVER_MACHINE, server_host), (CLIENT_MACHINE, client_host)):
        openwrt_env.sync_build_output(machine)
        openwrt_env.install_ipk_on_host(host, xp2p_openwrt_ipk)

    for host in (server_host, client_host):
        _update_hosts_entry(host, "add", SERVER_DOMAIN, SERVER_IP)
    try:
        server_install = server_runner(
            "server",
            "install",
            "--path",
            server_install_path,
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--host",
            SERVER_DOMAIN,
            "--force",
            check=True,
        )
        credential = helpers.extract_trojan_credential(server_install.stdout or "")
        assert SERVER_DOMAIN in credential["link"], "Expected domain in trojan link"
        reverse_tag = helpers.expected_reverse_tag(credential["user"], SERVER_DOMAIN)

        server_state = helpers.read_server_config(server_host)
        server_routing = helpers.read_json(server_host, helpers.SERVER_CONFIG_DIR / "routing.json")
        helpers.assert_server_reverse_state(
            server_state,
            reverse_tag,
            user=credential["user"],
            host=SERVER_DOMAIN,
        )
        helpers.assert_server_reverse_routing(server_routing, reverse_tag, user=credential["user"])

        client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--link",
            credential["link"],
            "--force",
            check=True,
        )
        client_state = helpers.read_client_config(client_host)
        client_routing = helpers.read_json(client_host, helpers.CLIENT_CONFIG_DIR / "routing.json")
        client_outbounds = helpers.read_json(client_host, helpers.CLIENT_CONFIG_DIR / "outbounds.json")
        endpoint_tag = helpers.expected_proxy_tag(SERVER_DOMAIN)
        helpers.assert_client_reverse_artifacts(client_routing, reverse_tag, endpoint_tag)
        helpers.assert_client_reverse_state(
            client_state,
            reverse_tag,
            endpoint_tag=endpoint_tag,
            user=credential["user"],
            host=SERVER_DOMAIN,
        )
        helpers.assert_outbound(
            client_outbounds,
            SERVER_DOMAIN,
            credential["password"],
            credential["user"],
            SERVER_DOMAIN,
            allow_insecure=True,
        )

        helpers.assert_reverse_cli_output(
            server_runner,
            "server",
            server_install_path,
            helpers.SERVER_CONFIG_DIR_NAME,
            reverse_tag,
        )
        helpers.assert_reverse_cli_output(
            client_runner,
            "client",
            helpers.INSTALL_ROOT,
            helpers.CLIENT_CONFIG_DIR_NAME,
            reverse_tag,
        )

        yield {
            "server_host": server_host,
            "client_host": client_host,
            "server_runner": server_runner,
            "client_runner": client_runner,
            "server_install_path": server_install_path,
            "reverse_tag": reverse_tag,
            "endpoint_tag": endpoint_tag,
            "client_primary_ip": client_primary_ip,
            "client_user": credential["user"],
        }
    finally:
        cleanup()


@contextmanager
def _active_tunnel_sessions(env: dict):
    with openwrt_env.xp2p_run_session(
        env["server_host"],
        "server",
        env["server_install_path"],
        helpers.SERVER_CONFIG_DIR_NAME,
        helpers.SERVER_LOG_FILE,
    ), openwrt_env.xp2p_run_session(
        env["client_host"],
        "client",
        helpers.INSTALL_ROOT.as_posix(),
        helpers.CLIENT_CONFIG_DIR_NAME,
        helpers.CLIENT_LOG_FILE,
    ):
        time.sleep(2.0)
        yield


def _server_forward_cmd(env: dict, subcommand: str, *extra: str, check: bool = False):
    args = [
        "server",
        "forward",
        subcommand,
        "--path",
        env["server_install_path"],
        "--config-dir",
        helpers.SERVER_CONFIG_DIR_NAME,
    ]
    if extra:
        args.extend(extra)
    return env["server_runner"](*args, check=check)


def _exercise_client_forward_diagnostics(env: dict) -> None:
    client_runner = env["client_runner"]
    client_host = env["client_host"]
    forward_target = f"{SERVER_DOMAIN}:{SERVER_DIAGNOSTICS_PORT}"
    listen_port = CLIENT_FORWARD_PORT
    try:
        client_runner(
            "client",
            "forward",
            "remove",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--listen-port",
            str(listen_port),
            check=False,
        )
        client_runner(
            "client",
            "forward",
            "add",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--target",
            forward_target,
            "--listen",
            "127.0.0.1",
            "--listen-port",
            str(listen_port),
            "--proto",
            "tcp",
            check=True,
        )
        client_state = helpers.read_client_config(client_host)
        for entry in client_state.get("forwards") or []:
            recorded_host = (entry.get("target_host") or "").strip()
            recorded_port = int(entry.get("target_port") or entry.get("targetPort") or 0)
            if recorded_host == SERVER_DOMAIN and recorded_port == SERVER_DIAGNOSTICS_PORT:
                listen_port = tunnel_common.listen_port_from_entry(entry)
                break
        assert listen_port == CLIENT_FORWARD_PORT

        with _active_tunnel_sessions(env):
            ping_result = client_runner(
                "ping",
                "127.0.0.1",
                "--port",
                str(listen_port),
                "--count",
                "3",
                "--proto",
                "tcp",
                check=True,
            )
            tunnel_common.assert_zero_loss(ping_result, f"via client forward on port {listen_port}")
    finally:
        if listen_port:
            client_runner(
                "client",
                "forward",
                "remove",
                "--path",
                helpers.INSTALL_ROOT.as_posix(),
                "--config-dir",
                helpers.CLIENT_CONFIG_DIR_NAME,
                "--listen-port",
                str(listen_port),
                check=False,
            )


def _exercise_server_forward_diagnostics(env: dict) -> None:
    server_host = env["server_host"]
    server_runner = env["server_runner"]
    forward_target = f"{CLIENT_IP}:{CLIENT_DIAGNOSTICS_PORT}"
    listen_port = None
    try:
        _server_forward_cmd(
            env,
            "add",
            "--target",
            forward_target,
            "--listen-port",
            str(SERVER_FORWARD_PORT),
            "--listen",
            "127.0.0.1",
            "--proto",
            "tcp",
            check=True,
        )
        server_state = helpers.read_server_config(server_host)
        entry = tunnel_common.forward_entry_for_target(
            server_state.get("forward_rules") or [], CLIENT_IP, CLIENT_DIAGNOSTICS_PORT
        )
        listen_port = tunnel_common.listen_port_from_entry(entry)

        with _active_tunnel_sessions(env):
            ping_result = server_runner(
                "ping",
                "127.0.0.1",
                "--port",
                str(listen_port),
                "--count",
                "3",
                "--proto",
                "tcp",
                check=True,
            )
            tunnel_common.assert_zero_loss(ping_result, f"via server forward on port {listen_port}")
    finally:
        if listen_port:
            _server_forward_cmd(
                env,
                "remove",
                "--listen-port",
                str(listen_port),
                check=False,
            )


def _verify_heartbeat_state(env: dict) -> None:
    expected_tag = env["endpoint_tag"]
    expected_user = env["client_user"]
    expected_client_ip = env["client_primary_ip"]
    server_install_path = env["server_install_path"]
    client_install_path = helpers.INSTALL_ROOT.as_posix()

    helpers.wait_for_heartbeat_state(env["server_host"], SERVER_HEARTBEAT_STATE_FILE)
    helpers.wait_for_heartbeat_state(env["client_host"], CLIENT_HEARTBEAT_STATE_FILE)
    try:
        tunnel_common.wait_for_alive_entry(
            env["server_runner"],
            "server",
            server_install_path,
            expected_tag,
            SERVER_DOMAIN,
            expected_user,
            expected_client_ip,
        )
    except AssertionError:
        state_output = env["server_runner"]("server", "state", "--path", server_install_path, check=True).stdout or ""
        rows = tunnel_common.parse_state_rows(state_output)
        assert any(row.get("TAG") == expected_tag for row in rows), "Heartbeat entry missing on server"
    try:
        tunnel_common.wait_for_alive_entry(
            env["client_runner"],
            "client",
            client_install_path,
            expected_tag,
            SERVER_DOMAIN,
            expected_user,
            expected_client_ip,
        )
    except AssertionError:
        state_output = env["client_runner"]("client", "state", "--path", client_install_path, check=True).stdout or ""
        rows = tunnel_common.parse_state_rows(state_output)
        assert any(row.get("TAG") == expected_tag for row in rows), "Heartbeat entry missing on client"


def _assert_state_uses_domain(env: dict) -> None:
    server_state = env["server_runner"]("server", "state", "--path", env["server_install_path"], check=True)
    client_state = env["client_runner"]("client", "state", "--path", helpers.INSTALL_ROOT.as_posix(), check=True)
    for output in (server_state.stdout or "", client_state.stdout or ""):
        rows = tunnel_common.parse_state_rows(output)
        assert any(
            row.get("HOST") == SERVER_DOMAIN and row.get("TAG") == env["endpoint_tag"]
            for row in rows
        ), f"xp2p state output does not reference {SERVER_DOMAIN}"


def test_forward_tunnel_uses_domain_name(tunnel_environment):
    client_runner = tunnel_environment["client_runner"]

    with _active_tunnel_sessions(tunnel_environment):
        ping_result = client_runner(
            "ping",
            SERVER_DOMAIN,
            "--tunnel",
            "--count",
            "3",
            check=True,
        )
        tunnel_common.assert_zero_loss(ping_result, "through SOCKS tunnel via domain")
        _verify_heartbeat_state(tunnel_environment)
        _assert_state_uses_domain(tunnel_environment)
    _exercise_client_forward_diagnostics(tunnel_environment)
    _exercise_server_forward_diagnostics(tunnel_environment)
