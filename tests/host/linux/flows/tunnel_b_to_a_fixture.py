from __future__ import annotations

from contextlib import contextmanager
from pathlib import PurePosixPath
import time

import pytest

from tests.host.host_common.polling import wait_until
from tests.host.linux import _helpers as helpers
from tests.host.linux import env as linux_env
from tests.host.linux import parsers
from tests.host.tunnel import common as tunnel_common

SERVER_IP = "10.62.10.11"  # deb-test-a (host A)
CLIENT_IP = "10.62.10.12"  # deb-test-b (host B)
CLIENT_REVERSE_TEST_IP = "10.0.102.50"
SERVER_DIAGNOSTICS_PORT = 62022
CLIENT_DIAGNOSTICS_PORT = 62023
SOCKS_PORT = 51180
SERVER_FORWARD_PORT = 53341
CLIENT_FORWARD_PORT = 53331
CLIENT_REDIRECT_CIDR = "10.0.101.0/24"
SERVER_REDIRECT_CIDR = "10.0.102.0/24"
SERVER_HEARTBEAT_STATE_FILE = helpers.SERVER_HEARTBEAT_STATE_FILE
CLIENT_HEARTBEAT_STATE_FILE = helpers.CLIENT_HEARTBEAT_STATE_FILE


def runner(host):
    def _run(*args: str, check: bool = False):
        cmd = list(args)
        pending_targets = {
            ("client", "list"),
            ("client", "forward", "list"),
            ("client", "reverse"),
            ("client", "reverse", "list"),
            ("server", "forward", "list"),
            ("server", "redirect", "list"),
            ("server", "reverse"),
            ("server", "reverse", "list"),
            ("server", "user", "list"),
            ("server", "cert", "state"),
        }
        if "--pending" not in cmd and "-y" not in cmd:
            if cmd[:2] == ["client", "redirect"] and (len(cmd) == 2 or cmd[2].startswith("-")):
                cmd.append("--pending")
            else:
                for target in pending_targets:
                    if tuple(cmd[: len(target)]) == target:
                        cmd.append("--pending")
                        break
        result = linux_env.run_xp2p(host, *cmd)
        if check and result.rc != 0:
            helpers.dump_failure_state(host, f"tunnel-B-to-A-runner-{host.backend.hostname}")
            pytest.fail(
                "xp2p command failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        return result

    return _run


def nat_runner(host, *, role: str):
    env = {"XP2P_CLIENT_TUN_ENABLED": "false", "XP2P_SERVER_TUN_ENABLED": "false"}

    def _run(*args: str, check: bool = False):
        result = linux_env.run_xp2p_with_env(host, env, *args)
        if check and result.rc != 0:
            helpers.dump_failure_state(host, f"tunnel-B-to-A-nat-{role}-{host.backend.hostname}")
            pytest.fail(
                "xp2p command failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        return result

    return _run


def verify_heartbeat_state(env: dict) -> None:
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
            SERVER_IP,
            expected_user,
            expected_client_ip,
        )
    except AssertionError:
        state_output = env["server_runner"]("server", "state", "--json", "--path", server_install_path, check=True).stdout or ""
        rows = tunnel_common.parse_state_result(state_output)
        assert any(row.get("TAG") == expected_tag for row in rows), "Heartbeat entry missing on server"
    try:
        tunnel_common.wait_for_alive_entry(
            env["client_runner"],
            "client",
            client_install_path,
            expected_tag,
            SERVER_IP,
            expected_user,
            expected_client_ip,
        )
    except AssertionError:
        state_output = env["client_runner"]("client", "state", "--json", "--path", client_install_path, check=True).stdout or ""
        rows = tunnel_common.parse_state_result(state_output)
        assert any(row.get("TAG") == expected_tag for row in rows), "Heartbeat entry missing on client"


def wait_for_dead_entry(
    env: dict,
    *,
    ttl: str = "3s",
    timeout_seconds: float = 45.0,
    poll_interval: float = 2.0,
) -> dict:
    expected_tag = env["endpoint_tag"]
    expected_host = SERVER_IP
    install_path = env["server_install_path"]

    def _poll():
        result = env["server_runner"](
            "server",
            "state",
            "--json",
            "--path",
            install_path,
            "--ttl",
            ttl,
            check=True,
        )
        stdout = result.stdout or ""
        for row in tunnel_common.parse_state_result(stdout):
            if row.get("TAG", "").strip() != expected_tag:
                continue
            if row.get("HOST", "").strip() != expected_host:
                continue
            if row.get("STATUS", "").strip().lower() in {"dead", "unhealthy"}:
                return row
        return None

    try:
        return wait_until(
            f"heartbeat entry {expected_tag}@{expected_host} to become dead",
            _poll,
            timeout_seconds=timeout_seconds,
            poll_interval=poll_interval,
        ).value
    except TimeoutError as exc:
        last_stdout = env["server_runner"](
            "server",
            "state", "--json",
            "--path",
            install_path,
            "--ttl",
            ttl,
            check=True,
        ).stdout or ""
        raise AssertionError(f"{exc}\nLast xp2p server state output:\n{last_stdout}") from exc


def run_server_state_watch(env: dict, duration_seconds: float = 7.0) -> None:
    server_host = env["server_host"]
    install_path = env["server_install_path"]
    xp2p_binary = linux_env.INSTALL_PATH.as_posix()
    timeout_arg = f"{duration_seconds:.0f}s"
    command = (
        f"timeout -k 1s {timeout_arg} {xp2p_binary} server state "
        f"--watch --interval 2s --path {install_path}"
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
        if raw.strip()
        and tuple(
            tunnel_common.split_state_line(
                raw.strip(),
                len(tunnel_common.STATE_TABLE_BASE_HEADER),
            )
        )
        == tunnel_common.STATE_TABLE_BASE_HEADER
    )
    assert header_count >= 2, "xp2p server state --watch did not refresh multiple times"
    assert header_count <= 5, "xp2p server state --watch produced unexpected amount of output"


def wait_for_port(host, port: int, *, timeout_seconds: float = 20.0, interval: float = 1.0) -> None:
    def _poll():
        check = host.run(f"sudo -n ss -lnt | grep -q ':{port} '")
        return True if check.rc == 0 else None

    try:
        wait_until(
            f"port {port} to open on {host.backend.hostname}",
            _poll,
            timeout_seconds=timeout_seconds,
            poll_interval=interval,
        )
    except TimeoutError as exc:
        pytest.fail(str(exc))


def wait_for_path(host, path: PurePosixPath, *, timeout_seconds: float = 45.0, interval: float = 1.0) -> None:
    def _poll():
        return True if linux_env.path_exists(host, path) else None

    try:
        wait_until(
            f"path {path} to exist on {host.backend.hostname}",
            _poll,
            timeout_seconds=timeout_seconds,
            poll_interval=interval,
        )
    except TimeoutError as exc:
        pytest.fail(str(exc))


def wait_for_apply_request_clear(host, *, timeout_seconds: float = 90.0, interval: float = 1.0) -> None:
    path = helpers.STATE_ROOT / "apply.request"

    def _poll():
        return True if not linux_env.path_exists(host, path) else None

    try:
        wait_until(
            f"{path} to be removed on {host.backend.hostname}",
            _poll,
            timeout_seconds=timeout_seconds,
            poll_interval=interval,
        )
    except TimeoutError as exc:
        pytest.fail(str(exc))


def socks_port(host, config_path: PurePosixPath) -> int:
    data = helpers.read_json(host, config_path)
    for inbound in data.get("inbounds", []) or []:
        if not isinstance(inbound, dict):
            continue
        if inbound.get("protocol") != "socks":
            continue
        port_val = inbound.get("port")
        if isinstance(port_val, int):
            return port_val
    return SOCKS_PORT


@pytest.fixture
def tunnel_environment(linux_host_factory, xp2p_full_cleanup):
    server_host = linux_host_factory(linux_env.DEFAULT_CLIENT)
    client_host = linux_host_factory(linux_env.DEFAULT_SERVER)
    server_runner = runner(server_host)
    client_runner = runner(client_host)
    server_install_path = helpers.INSTALL_ROOT.as_posix()
    client_primary_ip = helpers.detect_primary_ipv4(client_host)

    def cleanup():
        for host in (server_host, client_host):
            host.run("sudo -n xp2p client service stop --quiet >/dev/null 2>&1 || true")
            host.run("sudo -n xp2p server service stop --quiet >/dev/null 2>&1 || true")
            host.run("sudo -n /etc/init.d/xp2p stop >/dev/null 2>&1 || true")
            host.run(
                "for p in 62022 62023 52080 52180 51080 51180; "
                "do sudo -n fuser -k ${p}/tcp ${p}/udp >/dev/null 2>&1 || true; done"
            )
            host.run("sudo -n pkill -f '/usr/bin/xp2p' >/dev/null 2>&1 || true")
            host.run(f"sudo -n pkill -f {helpers.XRAY_BINARY.as_posix()!r} >/dev/null 2>&1 || true")
            host.run("sudo -n nft delete table inet xray_transparent >/dev/null 2>&1 || true")
            host.run(
                "sudo -n rm -f /etc/nftables.d/xray-transparent.nft /etc/xp2p/nftables/xray-transparent.nft >/dev/null 2>&1 || true"
            )
            host.run(
                "sudo -n rm -f /etc/nftables.d/xray-transparent.d/*.entry /etc/xp2p/nftables/xray-transparent.d/*.entry >/dev/null 2>&1 || true"
            )
        helpers.remove_path(server_host, SERVER_HEARTBEAT_STATE_FILE)
        helpers.remove_path(client_host, CLIENT_HEARTBEAT_STATE_FILE)

    cleanup()
    try:
        server_install = server_runner(
            "server",
            "install", "--json",
            "--path",
            server_install_path,
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--host",
            SERVER_IP,
            "--force",
            check=True,
        )
        credential = helpers.parse_json_credential(server_install.stdout or "")
        assert credential["link"], "Expected connection link in server install output"
        reverse_tag = helpers.expected_reverse_tag(credential["user"], SERVER_IP)

        server_state = helpers.read_pending_server_config(server_host)
        server_routing = helpers.render_xray(server_host, server_runner, "server", desired=True)
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
            "--mode",
            "proxy",
            "--link",
            credential["link"],
            "--force",
            check=True,
        )
        client_state = helpers.read_pending_client_config(client_host)
        client_routing = helpers.render_xray(client_host, client_runner, "client", desired=True)
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
            "client_password": credential["password"],
            "client_link": credential["link"],
        }
    finally:
        cleanup()


@contextmanager
def active_tunnel_sessions(
    env: dict,
    *,
    runtime_metrics: bool = False,
    test_heartbeat_interval: str = "",
):
    server_metrics = "/tmp/xp2p-server-runtime.metrics" if runtime_metrics else ""
    client_metrics = "/tmp/xp2p-client-runtime.metrics" if runtime_metrics else ""
    with linux_env.xp2p_run_session(
        env["server_host"],
        "server",
        env["server_install_path"],
        helpers.SERVER_CONFIG_DIR_NAME,
        runtime_metrics_file=server_metrics,
    ) as server_session, linux_env.xp2p_run_session(
        env["client_host"],
        "client",
        helpers.INSTALL_ROOT.as_posix(),
        helpers.CLIENT_CONFIG_DIR_NAME,
        runtime_metrics_file=client_metrics,
        test_heartbeat_interval=test_heartbeat_interval,
    ) as client_session:
        time.sleep(2.0)
        wait_for_apply_request_clear(env["server_host"])
        wait_for_apply_request_clear(env["client_host"])
        wait_for_path(env["server_host"], helpers.SERVER_LIVE_DIR / "xray.json")
        wait_for_path(env["server_host"], helpers.SERVER_LIVE_DIR / "runtime.json")
        wait_for_path(env["client_host"], helpers.CLIENT_LIVE_DIR / "xray.json")
        wait_for_path(env["client_host"], helpers.CLIENT_LIVE_DIR / "runtime.json")
        server_socks_port = socks_port(env["server_host"], helpers.SERVER_LIVE_DIR / "xray.json")
        client_socks_port = socks_port(env["client_host"], helpers.CLIENT_LIVE_DIR / "xray.json")
        wait_for_port(env["server_host"], server_socks_port)
        wait_for_port(env["client_host"], client_socks_port)
        yield {
            "server": {**server_session, "runtime_metrics": server_metrics},
            "client": {**client_session, "runtime_metrics": client_metrics},
        }


def server_forward_cmd(env: dict, subcommand: str, *extra: str, check: bool = False):
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


@contextmanager
def ip_alias(host, cidr: str, dev: str = "lo"):
    add_cmd = f"sudo -n ip addr add {cidr} dev {dev}"
    add_result = host.run(add_cmd)
    if add_result.rc != 0 and "file exists" not in (add_result.stderr or "").lower():
        pytest.fail(
            f"Failed to add IP alias {cidr} on {dev}.\n"
            f"CMD: {add_cmd}\nSTDOUT:\n{add_result.stdout}\nSTDERR:\n{add_result.stderr}"
        )
    try:
        yield
    finally:
        host.run(f"sudo -n ip addr del {cidr} dev {dev} >/dev/null 2>&1 || true")


def exercise_client_forward_diagnostics(env: dict) -> None:
    client_runner = env["client_runner"]
    client_host = env["client_host"]
    forward_target = f"{SERVER_IP}:{SERVER_DIAGNOSTICS_PORT}"
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
        client_state = helpers.read_pending_client_config(client_host)
        entry = tunnel_common.forward_entry_for_target(
            client_state.get("forwards") or [], SERVER_IP, SERVER_DIAGNOSTICS_PORT
        )
        listen_port = tunnel_common.listen_port_from_entry(entry)
        assert listen_port == CLIENT_FORWARD_PORT

        with active_tunnel_sessions(env):
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


def exercise_server_forward_diagnostics(env: dict) -> None:
    server_host = env["server_host"]
    server_runner = env["server_runner"]
    forward_target = f"{CLIENT_IP}:{CLIENT_DIAGNOSTICS_PORT}"
    listen_port = None
    try:
        server_forward_cmd(
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
        server_state = helpers.read_pending_server_config(server_host)
        entry = tunnel_common.forward_entry_for_target(
            server_state.get("forward_rules") or [], CLIENT_IP, CLIENT_DIAGNOSTICS_PORT
        )
        listen_port = tunnel_common.listen_port_from_entry(entry)

        with active_tunnel_sessions(env):
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
            server_forward_cmd(
                env,
                "remove",
                "--listen-port",
                str(listen_port),
                check=False,
            )


def assert_redirect_entries_removed(
    env: dict,
    *,
    client_cidr: str,
    client_tag: str,
    server_cidr: str,
    server_tag: str,
) -> None:
    client_runner = env["client_runner"]
    server_runner = env["server_runner"]
    server_install_path = env["server_install_path"]

    client_redirect_list = client_runner(
        "client",
        "redirect",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
        "--json",
        check=True,
    ).stdout or ""
    server_redirect_list = server_runner(
        "server",
        "redirect",
        "list",
        "--path",
        server_install_path,
        "--config-dir",
        helpers.SERVER_CONFIG_DIR_NAME,
        "--json",
        check=True,
    ).stdout or ""

    client_entries = parsers.parse_redirect_output(client_redirect_list)
    server_entries = parsers.parse_redirect_output(server_redirect_list)
    if parsers.has_redirect_entry(server_entries, cidr=server_cidr, tag=server_tag):
        raise AssertionError(
            f"Server redirect list still contains {server_cidr} for {server_tag}:\n{server_redirect_list}"
        )
    if parsers.has_redirect_entry(client_entries, cidr=client_cidr, tag=client_tag):
        raise AssertionError(
            f"Client redirect output still contains {client_cidr} for {client_tag}:\n{client_redirect_list}"
        )
