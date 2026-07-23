from __future__ import annotations

from contextlib import contextmanager
import shlex
import time

import pytest

from tests.host.openwrt import _helpers as helpers
from tests.host.tunnel import common as tunnel_common
from tests.host.openwrt.flows import tunnel_b_to_a_waits as waits


def dump_run_logs(host, role: str, stage: str) -> None:
    log_path = f"/var/log/xp2p/{role}/service.log"
    run_log_path = f"/tmp/xp2p-{role}-run.log"
    host.run(
        "sh -c "
        + shlex.quote(
            f"echo '--- {role} service log ({stage}) ---'; "
            f"if [ -f {log_path} ]; then tail -n 200 {log_path}; else echo 'missing {log_path}'; fi; "
            f"echo '--- {role} run log ({stage}) ---'; "
            f"if [ -f {run_log_path} ]; then tail -n 200 {run_log_path}; else echo 'missing {run_log_path}'; fi"
        )
    )


def dump_apply_state(host, role: str, stage: str) -> None:
    config_dir = helpers.CLIENT_CONFIG_DIR if role == "client" else helpers.SERVER_CONFIG_DIR
    host.run(
        "sh -c "
        + shlex.quote(
            " ; ".join(
                (
                    f"echo '--- {role} apply state ({stage}) ---'",
                    "ls -la /etc/xp2p/.state 2>/dev/null || echo 'missing /etc/xp2p/.state'",
                    "ls -la /etc/xp2p/.state/pending 2>/dev/null || echo 'missing /etc/xp2p/.state/pending'",
                    f"ls -la {config_dir.as_posix()} 2>/dev/null || echo 'missing {config_dir.as_posix()}'",
                    f"if [ -f /etc/xp2p/xp2p-{role}.toml ]; then echo 'live xp2p-{role}.toml: present'; else echo 'live xp2p-{role}.toml: missing'; fi",
                    "if [ -f /etc/xp2p/.state/apply.request ]; then echo 'apply.request: present'; else echo 'apply.request: missing'; fi",
                    f"/usr/bin/xp2p {role} state --path /etc/xp2p 2>/dev/null || true",
                    f"if [ -f /var/log/xp2p/{role}/service.log ]; then tail -n 200 /var/log/xp2p/{role}/service.log; else echo '/var/log/xp2p/{role}/service.log missing'; fi",
                )
            )
        )
    )


def assert_server_alias_reachable(host, runner, target_ip: str, port: int) -> None:
    ping_result = runner(
        "ping",
        target_ip,
        "--port",
        str(port),
        "--count",
        "1",
        "--proto",
        "tcp",
        check=False,
    )
    if ping_result.rc == 0:
        return
    ip_addr = host.run("ip addr show").stdout or ""
    routes = host.run("ip route show").stdout or ""
    listeners = host.run("netstat -lpn 2>/dev/null | grep ':62022 ' || true").stdout or ""
    pytest.fail(
        "Server alias diagnostics ping failed.\n"
        f"target={target_ip}:{port}\n"
        f"STDOUT:\n{ping_result.stdout}\nSTDERR:\n{ping_result.stderr}\n"
        f"ip addr:\n{ip_addr}\n"
        f"ip route:\n{routes}\n"
        f"listeners:\n{listeners}\n"
    )


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


def exercise_client_forward_diagnostics(env: dict, active_tunnel_sessions) -> None:
    client_runner = env["client_runner"]
    client_host = env["client_host"]
    forward_target = f"{waits.SERVER_IP}:{waits.SERVER_DIAGNOSTICS_PORT}"
    listen_port = waits.CLIENT_FORWARD_PORT
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
        waits.apply_pending_config_wait(
            client_host,
            "client",
            helpers.INSTALL_ROOT.as_posix(),
            helpers.CLIENT_CONFIG_DIR_NAME,
        )
        with active_tunnel_sessions(env):
            client_state = helpers.read_live_client_config(client_host)
            entry = tunnel_common.forward_entry_for_target(
                client_state.get("forwards") or [], waits.SERVER_IP, waits.SERVER_DIAGNOSTICS_PORT
            )
            listen_port = tunnel_common.listen_port_from_entry(entry)
            assert listen_port == waits.CLIENT_FORWARD_PORT
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


def exercise_server_forward_diagnostics(env: dict, active_tunnel_sessions) -> None:
    server_host = env["server_host"]
    server_runner = env["server_runner"]
    forward_target = f"{waits.CLIENT_IP}:{waits.CLIENT_DIAGNOSTICS_PORT}"
    listen_port = None
    try:
        server_forward_cmd(
            env,
            "add",
            "--target",
            forward_target,
            "--listen-port",
            str(waits.SERVER_FORWARD_PORT),
            "--listen",
            "127.0.0.1",
            "--proto",
            "tcp",
            check=True,
        )
        waits.apply_pending_config_wait(
            server_host,
            "server",
            env["server_install_path"],
            helpers.SERVER_CONFIG_DIR_NAME,
        )
        with active_tunnel_sessions(env):
            server_state = helpers.read_live_server_config(server_host)
            entry = tunnel_common.forward_entry_for_target(
                server_state.get("forward_rules") or [], waits.CLIENT_IP, waits.CLIENT_DIAGNOSTICS_PORT
            )
            listen_port = tunnel_common.listen_port_from_entry(entry)
            waits.wait_for_listen_port(server_host, listen_port)
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


def verify_heartbeat_state(env: dict) -> None:
    expected_tag = env["endpoint_tag"]
    expected_user = env["client_user"]
    expected_client_ip = env["client_primary_ip"]
    server_install_path = env["server_install_path"]
    client_install_path = helpers.INSTALL_ROOT.as_posix()

    if not helpers.path_exists_live(env["server_host"], waits.SERVER_HEARTBEAT_STATE_FILE):
        pytest.fail("Server heartbeat state not found after run start")
    if not helpers.path_exists_live(env["client_host"], waits.CLIENT_HEARTBEAT_STATE_FILE):
        pytest.fail("Client heartbeat state not found after run start")

    server_state = helpers.wait_for_heartbeat_state(
        env["server_host"],
        waits.SERVER_HEARTBEAT_STATE_FILE,
        timeout_seconds=20.0,
        poll_interval=2.0,
    )
    server_entry = helpers.assert_heartbeat_entry(
        server_state,
        expected_tag,
        host=waits.SERVER_IP,
        user=expected_user,
    )
    recorded_ip = (server_entry.get("client_ip") or "").strip()
    if recorded_ip:
        expected_client_ip = recorded_ip

    try:
        tunnel_common.wait_for_alive_entry(
            env["server_runner"],
            "server",
            server_install_path,
            expected_tag,
            waits.SERVER_IP,
            expected_user,
            expected_client_ip,
            timeout_seconds=20.0,
            poll_interval=2.0,
        )
    except AssertionError:
        state_output = env["server_runner"]("server", "state", "--path", server_install_path, check=True).stdout or ""
        raise AssertionError(
            f"server heartbeat entry not found for {expected_tag}@{waits.SERVER_IP}.\nState output:\n{state_output}"
        ) from None

    try:
        tunnel_common.wait_for_alive_entry(
            env["client_runner"],
            "client",
            client_install_path,
            expected_tag,
            waits.SERVER_IP,
            expected_user,
            expected_client_ip,
            timeout_seconds=20.0,
            poll_interval=2.0,
        )
    except AssertionError:
        state_output = env["client_runner"]("client", "state", "--path", client_install_path, check=True).stdout or ""
        raise AssertionError(
            f"client heartbeat entry not found for {expected_tag}@{waits.SERVER_IP}.\nState output:\n{state_output}"
        ) from None


def run_server_state_watch(env: dict, duration_seconds: float = 7.0) -> None:
    server_host = env["server_host"]
    install_path = env["server_install_path"]
    timeout_arg = f"{duration_seconds:.0f}"
    command = (
        "sh -c '"
        "tmp=$(mktemp); "
        f"xp2p server state --watch --interval 2s --path {shlex.quote(install_path)} >\"$tmp\" 2>&1 & "
        "pid=$!; "
        f"sleep {timeout_arg}; "
        "kill -INT $pid >/dev/null 2>&1 || true; "
        "wait $pid >/dev/null 2>&1 || true; "
        "cat \"$tmp\"; rm -f \"$tmp\"'"
    )
    result = server_host.run(command)
    if result.rc != 0:
        pytest.fail(
            "xp2p server state --watch failed "
            f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    cleaned = tunnel_common.strip_ansi(result.stdout or "")
    header_count = sum(
        1
        for raw in cleaned.splitlines()
        if tuple(
            tunnel_common.split_state_line(
                raw.strip(),
                len(tunnel_common.STATE_TABLE_BASE_HEADER),
            )
        )
        == tunnel_common.STATE_TABLE_BASE_HEADER
    )
    assert header_count >= 2, "xp2p server state --watch did not refresh multiple times"
    assert header_count <= 5, "xp2p server state --watch produced unexpected amount of output"
