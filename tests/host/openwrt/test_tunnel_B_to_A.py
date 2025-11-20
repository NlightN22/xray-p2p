from __future__ import annotations

from contextlib import contextmanager

import pytest

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env
from tests.host.tunnel import common as tunnel_common

SERVER_MACHINE = openwrt_env.OPENWRT_MACHINES[0]
CLIENT_MACHINE = openwrt_env.OPENWRT_MACHINES[1]
SERVER_IP = "10.63.30.11"
CLIENT_IP = "10.63.30.12"
CLIENT_REVERSE_TEST_IP = "10.0.102.50"
DIAGNOSTICS_PORT = 62022
SERVER_FORWARD_PORT = 53341
CLIENT_REDIRECT_CIDR = "10.0.102.0/24"
pytestmark = [pytest.mark.host, pytest.mark.linux]
HEARTBEAT_STATE_FILE = helpers.HEARTBEAT_STATE_FILE


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


@pytest.fixture(scope="module")
def tunnel_environment(openwrt_server_host, openwrt_client_host, xp2p_openwrt_ipk):
    server_host = openwrt_server_host
    client_host = openwrt_client_host
    server_runner = _runner(server_host)
    client_runner = _runner(client_host)
    server_install_path = helpers.INSTALL_ROOT.as_posix()
    client_primary_ip = helpers.detect_primary_ipv4(client_host)

    def cleanup():
        for host in (server_host, client_host):
            host.run("pkill -f 'xp2p server run' >/dev/null 2>&1 || true")
            host.run("pkill -f 'xp2p client run' >/dev/null 2>&1 || true")
            host.run("pkill -f '/etc/xp2p/bin/xray' >/dev/null 2>&1 || true")
        helpers.cleanup_server_install(server_host, server_runner)
        helpers.cleanup_client_install(client_host, client_runner)
        for host in (server_host, client_host):
            helpers.remove_path(host, HEARTBEAT_STATE_FILE)

    cleanup()
    openwrt_env.sync_build_output(SERVER_MACHINE)
    openwrt_env.install_ipk_on_host(server_host, xp2p_openwrt_ipk)
    openwrt_env.sync_build_output(CLIENT_MACHINE)
    openwrt_env.install_ipk_on_host(client_host, xp2p_openwrt_ipk)
    try:
        server_install = server_runner(
            "server",
            "install",
            "--path",
            server_install_path,
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--host",
            SERVER_IP,
            "--force",
            check=True,
        )
        credential = helpers.extract_trojan_credential(server_install.stdout or "")
        assert credential["link"], "Expected trojan link in server install output"
        reverse_tag = helpers.expected_reverse_tag(credential["user"], SERVER_IP)

        server_state = helpers.read_first_existing_json(server_host, helpers.SERVER_STATE_FILES)
        server_routing = helpers.read_json(server_host, helpers.SERVER_CONFIG_DIR / "routing.json")
        helpers.assert_server_reverse_state(
            server_state,
            reverse_tag,
            user=credential["user"],
            host=SERVER_IP,
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
        client_state = helpers.read_first_existing_json(client_host, helpers.CLIENT_STATE_FILES)
        client_routing = helpers.read_json(client_host, helpers.CLIENT_CONFIG_DIR / "routing.json")
        endpoint_tag = helpers.expected_proxy_tag(SERVER_IP)
        helpers.assert_client_reverse_artifacts(client_routing, reverse_tag, endpoint_tag)
        helpers.assert_client_reverse_state(
            client_state,
            reverse_tag,
            endpoint_tag=endpoint_tag,
            user=credential["user"],
            host=SERVER_IP,
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
        args.append("--")
        args.extend(extra)
    return env["server_runner"](*args, check=check)


@contextmanager
def _ip_alias(host, cidr: str, dev: str = "lo"):
    add_cmd = f"ip addr add {cidr} dev {dev}"
    add_result = host.run(add_cmd)
    if add_result.rc != 0 and "file exists" not in (add_result.stderr or "").lower():
        pytest.fail(
            f"Failed to add IP alias {cidr} on {dev}.\n"
            f"CMD: {add_cmd}\nSTDOUT:\n{add_result.stdout}\nSTDERR:\n{add_result.stderr}"
        )
    try:
        yield
    finally:
        host.run(f"ip addr del {cidr} dev {dev} >/dev/null 2>&1 || true")


def _exercise_client_forward_diagnostics(env: dict) -> None:
    client_runner = env["client_runner"]
    client_host = env["client_host"]
    forward_target = f"{SERVER_IP}:{DIAGNOSTICS_PORT}"
    listen_port = None
    try:
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
            "--proto",
            "tcp",
            check=True,
        )
        client_state = helpers.read_first_existing_json(client_host, helpers.CLIENT_STATE_FILES)
        entry = tunnel_common.forward_entry_for_target(client_state.get("forwards") or [], SERVER_IP, DIAGNOSTICS_PORT)
        listen_port = tunnel_common.listen_port_from_entry(entry)

        with _active_tunnel_sessions(env):
            ping_result = tunnel_common.ping_with_retries(
                client_runner,
                (
                    "ping",
                    "127.0.0.1",
                    "--port",
                    str(listen_port),
                    "--count",
                    "3",
                    "--proto",
                    "tcp",
                ),
                f"via client forward on port {listen_port}",
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
                "--ignore-missing",
                check=True,
            )


def _exercise_server_forward_diagnostics(env: dict) -> None:
    server_host = env["server_host"]
    server_runner = env["server_runner"]
    forward_target = f"{CLIENT_IP}:{DIAGNOSTICS_PORT}"
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
        server_state = helpers.read_first_existing_json(server_host, helpers.SERVER_STATE_FILES)
        entry = tunnel_common.forward_entry_for_target(server_state.get("forward_rules") or [], CLIENT_IP, DIAGNOSTICS_PORT)
        listen_port = tunnel_common.listen_port_from_entry(entry)

        with _active_tunnel_sessions(env):
            ping_result = tunnel_common.ping_with_retries(
                server_runner,
                (
                    "ping",
                    "127.0.0.1",
                    "--port",
                    str(listen_port),
                    "--count",
                    "3",
                    "--proto",
                    "tcp",
                ),
                f"via server forward on port {listen_port}",
            )
            tunnel_common.assert_zero_loss(ping_result, f"via server forward on port {listen_port}")
    finally:
        if listen_port:
            _server_forward_cmd(
                env,
                "remove",
                "--listen-port",
                str(listen_port),
                "--ignore-missing",
                check=True,
            )


def _verify_heartbeat_state(env: dict) -> None:
    expected_tag = env["endpoint_tag"]
    expected_user = env["client_user"]
    expected_client_ip = env["client_primary_ip"]
    server_install_path = env["server_install_path"]
    client_install_path = helpers.INSTALL_ROOT.as_posix()

    helpers.wait_for_heartbeat_state(env["server_host"], HEARTBEAT_STATE_FILE)
    helpers.wait_for_heartbeat_state(env["client_host"], HEARTBEAT_STATE_FILE)
    tunnel_common.wait_for_alive_entry(
        env["server_runner"],
        "server",
        server_install_path,
        expected_tag,
        SERVER_IP,
        expected_user,
        expected_client_ip,
    )
    tunnel_common.wait_for_alive_entry(
        env["client_runner"],
        "client",
        client_install_path,
        expected_tag,
        SERVER_IP,
        expected_user,
        expected_client_ip,
    )


def _run_server_state_watch(env: dict, duration_seconds: float = 7.0) -> None:
    server_host = env["server_host"]
    install_path = env["server_install_path"]
    timeout_arg = f"{duration_seconds:.0f}"
    command = (
        f"busybox timeout -s TERM {timeout_arg} "
        f"xp2p server state --watch --interval 2s --path {install_path}"
    )
    result = server_host.run(command)
    if result.rc not in (0, 124):
        pytest.fail(
            "xp2p server state --watch failed "
            f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    cleaned = tunnel_common.strip_ansi(result.stdout or "")
    header_count = sum(
        1
        for raw in cleaned.splitlines()
        if tuple(tunnel_common.split_state_line(raw.strip())) == tunnel_common.STATE_TABLE_HEADER
    )
    assert header_count >= 2, "xp2p server state --watch did not refresh multiple times"
    assert header_count <= 5, "xp2p server state --watch produced unexpected amount of output"


def test_forward_tunnel_operational(tunnel_environment):
    client_runner = tunnel_environment["client_runner"]

    with _active_tunnel_sessions(tunnel_environment):
        ping_result = tunnel_common.ping_with_retries(
            client_runner,
            (
                "ping",
                SERVER_IP,
                "--socks",
                "--count",
                "3",
            ),
            "through SOCKS tunnel",
        )
        tunnel_common.assert_zero_loss(ping_result, "through SOCKS tunnel")
        _verify_heartbeat_state(tunnel_environment)
        _run_server_state_watch(tunnel_environment)
    _exercise_client_forward_diagnostics(tunnel_environment)
    _exercise_server_forward_diagnostics(tunnel_environment)


def test_client_redirect_through_server(tunnel_environment):
    client_runner = tunnel_environment["client_runner"]
    client_host = tunnel_environment["client_host"]
    endpoint_tag = tunnel_environment["endpoint_tag"]

    client_runner(
        "client",
        "redirect",
        "add",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
        "--cidr",
        CLIENT_REDIRECT_CIDR,
        "--tag",
        endpoint_tag,
        check=True,
    )
    redirect_added = True
    try:
        client_state = helpers.read_first_existing_json(client_host, helpers.CLIENT_STATE_FILES)
        client_routing = helpers.read_json(client_host, helpers.CLIENT_CONFIG_DIR / "routing.json")
        helpers.assert_redirect_rule(client_routing, CLIENT_REDIRECT_CIDR, endpoint_tag)
        helpers.assert_client_reverse_state(
            client_state,
            tunnel_environment["reverse_tag"],
            endpoint_tag=endpoint_tag,
            user=tunnel_environment["client_user"],
            host=SERVER_IP,
        )
        with _active_tunnel_sessions(tunnel_environment):
            ping_result = tunnel_common.ping_with_retries(
                client_runner,
                (
                    "ping",
                    SERVER_IP,
                    "--count",
                    "3",
                    "--proto",
                    "tcp",
                ),
                "while redirecting through server",
            )
            tunnel_common.assert_zero_loss(ping_result, "while redirecting through server")
    finally:
        if redirect_added:
            client_runner(
                "client",
                "redirect",
                "remove",
                "--path",
                helpers.INSTALL_ROOT.as_posix(),
                "--config-dir",
                helpers.CLIENT_CONFIG_DIR_NAME,
                "--cidr",
                CLIENT_REDIRECT_CIDR,
                "--tag",
                endpoint_tag,
                "--ignore-missing",
                check=True,
            )
        final_list = client_runner(
            "client",
            "redirect",
            "list",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            check=True,
        ).stdout or ""
        assert CLIENT_REDIRECT_CIDR not in final_list


def test_reverse_redirect_via_server_portal(tunnel_environment):
    server_runner = tunnel_environment["server_runner"]
    server_install_path = tunnel_environment["server_install_path"]
    reverse_tag = tunnel_environment["reverse_tag"]
    client_host = tunnel_environment["client_host"]
    server_host = tunnel_environment["server_host"]

    alias_cidr = f"{CLIENT_REVERSE_TEST_IP}/32"
    with _ip_alias(client_host, alias_cidr):
        server_runner(
            "server",
            "redirect",
            "add",
            "--path",
            server_install_path,
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--cidr",
            alias_cidr,
            "--tag",
            reverse_tag,
            check=True,
        )
        forward_added = False
        try:
            list_output = server_runner(
                "server",
                "redirect",
                "list",
                "--path",
                server_install_path,
                "--config-dir",
                helpers.SERVER_CONFIG_DIR_NAME,
                check=True,
            ).stdout or ""
            assert alias_cidr in list_output, f"Server redirect list missing {alias_cidr}"

            server_state = helpers.read_first_existing_json(server_host, helpers.SERVER_STATE_FILES)
            server_routing = helpers.read_json(server_host, helpers.SERVER_CONFIG_DIR / "routing.json")
            helpers.assert_server_redirect_state(server_state, alias_cidr, reverse_tag)
            helpers.assert_server_redirect_rule(server_routing, alias_cidr, reverse_tag)

            server_runner(
                "server",
                "forward",
                "add",
                "--path",
                server_install_path,
                "--config-dir",
                helpers.SERVER_CONFIG_DIR_NAME,
                "--target",
                f"{CLIENT_REVERSE_TEST_IP}:{DIAGNOSTICS_PORT}",
                "--listen",
                "127.0.0.1",
                "--listen-port",
                str(SERVER_FORWARD_PORT),
                "--proto",
                "tcp",
                check=True,
            )
            forward_added = True

            server_state = helpers.read_first_existing_json(server_host, helpers.SERVER_STATE_FILES)
            entry = tunnel_common.forward_entry_for_target(
                server_state.get("forward_rules") or [], CLIENT_REVERSE_TEST_IP, DIAGNOSTICS_PORT
            )
            listen_port = tunnel_common.listen_port_from_entry(entry)
            assert listen_port == SERVER_FORWARD_PORT

            with _active_tunnel_sessions(tunnel_environment):
                ping_result = tunnel_common.ping_with_retries(
                    server_runner,
                    (
                        "ping",
                        "127.0.0.1",
                        "--port",
                        str(SERVER_FORWARD_PORT),
                        "--count",
                        "3",
                    ),
                    f"via server forward targeting {CLIENT_REVERSE_TEST_IP}",
                )
                tunnel_common.assert_zero_loss(ping_result, f"via server forward targeting {CLIENT_REVERSE_TEST_IP}")
        finally:
            if forward_added:
                _server_forward_cmd(
                    tunnel_environment,
                    "remove",
                    "--listen-port",
                    str(SERVER_FORWARD_PORT),
                    "--ignore-missing",
                    check=True,
                )
            server_runner(
                "server",
                "redirect",
                "remove",
                "--path",
                server_install_path,
                "--config-dir",
                helpers.SERVER_CONFIG_DIR_NAME,
                "--cidr",
                alias_cidr,
                "--tag",
                reverse_tag,
                check=True,
            )
            final_list = server_runner(
                "server",
                "redirect",
                "list",
                "--path",
                server_install_path,
                "--config-dir",
                helpers.SERVER_CONFIG_DIR_NAME,
                check=True,
            ).stdout or ""
            assert alias_cidr not in final_list
