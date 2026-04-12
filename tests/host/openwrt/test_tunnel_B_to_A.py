from __future__ import annotations

from contextlib import contextmanager
import shlex
import time

import pytest

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env
from tests.host.tunnel import common as tunnel_common

SERVER_MACHINE = openwrt_env.OPENWRT_MACHINES[0]
CLIENT_MACHINE = openwrt_env.OPENWRT_MACHINES[1]
SERVER_IP = "10.63.30.11"
CLIENT_IP = "10.63.30.12"
CLIENT_REVERSE_TEST_IP = "10.0.102.50"
SERVER_DIAGNOSTICS_PORT = 62022
CLIENT_DIAGNOSTICS_PORT = 62023
SERVER_FORWARD_PORT = 53341
CLIENT_FORWARD_PORT = 53331
CLIENT_REDIRECT_CIDR = "10.0.101.0/24"
SERVER_REDIRECT_CIDR = "10.0.102.0/24"
CLIENT_SOCKS_PORT = 51180
pytestmark = [pytest.mark.host, pytest.mark.linux]
SERVER_HEARTBEAT_STATE_FILE = helpers.SERVER_HEARTBEAT_STATE_FILE
CLIENT_HEARTBEAT_STATE_FILE = helpers.CLIENT_HEARTBEAT_STATE_FILE
REQUIRED_XRAY_CONFIGS = ("inbounds.json", "logs.json", "outbounds.json", "routing.json")


def _runner(host):
    def _run(*args: str, check: bool = False):
        result = openwrt_env.run_xp2p_live(host, *args)
        if check and result.rc != 0:
            pytest.fail(
                "xp2p command failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        return result

    return _run


def _xray_configs_missing(host, config_dir) -> list[str]:
    return [
        (config_dir / name).as_posix()
        for name in REQUIRED_XRAY_CONFIGS
        if not helpers.path_exists_live(host, config_dir / name)
    ]


def _wait_for_xray_configs(
    host,
    config_dir,
    *,
    timeout_seconds: float = 30.0,
    interval: float = 1.0,
) -> None:
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        missing = _xray_configs_missing(host, config_dir)
        if not missing:
            return
        time.sleep(interval)
    missing = _xray_configs_missing(host, config_dir)
    raise AssertionError(f"Missing xray configs (live): {missing}")


def _wait_for_live_config(
    host,
    role: str,
    *,
    timeout_seconds: float = 30.0,
    interval: float = 1.0,
) -> None:
    helpers.wait_for_live_config(
        host,
        role,
        timeout_seconds=timeout_seconds,
        poll_interval=interval,
    )


def _wait_for_service_state(
    host,
    role: str,
    expected_active: bool,
    *,
    timeout_seconds: float = 45.0,
    interval: float = 1.5,
) -> None:
    deadline = time.time() + timeout_seconds
    script = f"/etc/init.d/xp2p-{role}"
    last = None
    while time.time() < deadline:
        result = host.run(f"{script} running")
        active = result.rc == 0
        if active == expected_active:
            return
        last = result
        time.sleep(interval)
    stdout = getattr(last, "stdout", "") or ""
    stderr = getattr(last, "stderr", "") or ""
    state = "active" if expected_active else "inactive"
    raise AssertionError(
        f"xp2p {role} service did not reach {state} state.\nSTDOUT:\n{stdout}\nSTDERR:\n{stderr}"
    )


def _ensure_service_running(host, role: str) -> None:
    if _is_xp2p_run_active(host, role):
        return
    start = host.run(f"/etc/init.d/xp2p-{role} start")
    if start.rc != 0:
        pytest.fail(
            "Failed to start service "
            f"xp2p-{role} on {host.backend.hostname}.\nSTDOUT:\n{start.stdout}\nSTDERR:\n{start.stderr}"
        )
    _wait_for_service_state(host, role, expected_active=True)


def _current_mode(host, role: str) -> str:
    helpers.wait_for_live_config(host, role)
    if role == "client":
        config = helpers.read_live_client_config(host)
    elif role == "server":
        config = helpers.read_live_server_config(host)
    else:
        raise ValueError(f"Unsupported role: {role}")
    tun_enabled = config.get("tun_enabled")
    if not isinstance(tun_enabled, bool):
        raise AssertionError(f"Expected tun_enabled boolean in {role} config, got {tun_enabled!r}")
    return "tun" if tun_enabled else "proxy"


def _set_mode(runner, role: str, config_dir: str, mode: str) -> None:
    runner(
        role,
        "mode",
        mode,
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        config_dir,
        check=True,
    )


def _ensure_mode(host, runner, role: str, config_dir: str, mode: str) -> str:
    current = _current_mode(host, role)
    if current != mode:
        _set_mode(runner, role, config_dir, mode)
    return current


def _apply_pending_config(host, role: str, install_path: str, config_dir: str) -> None:
    helpers.apply_pending_config(host, role)


def _apply_pending_config_wait(host, role: str, install_path: str, config_dir: str) -> None:
    _apply_pending_config(host, role, install_path, config_dir)


def _is_xp2p_run_active(host, role: str) -> bool:
    cmd = (
        "ps w | "
        "grep -E "
        + shlex.quote(rf"xp2p {role} (run|service run)")
        + " | grep -v grep >/dev/null 2>&1"
    )
    return host.run(cmd).rc == 0


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
            openwrt_env._stop_xp2p_services(host)
            host.run("pkill -f 'xp2p server run' >/dev/null 2>&1 || true")
            host.run("pkill -f 'xp2p client run' >/dev/null 2>&1 || true")
            host.run("pkill -f '/etc/xp2p/bin/xray' >/dev/null 2>&1 || true")
            host.run("nft delete table inet xray_transparent >/dev/null 2>&1 || true")
            host.run("rm -f /etc/nftables.d/xray-transparent.nft /etc/xp2p/nftables/xray-transparent.nft >/dev/null 2>&1 || true")
            host.run(
                "rm -f /etc/nftables.d/xray-transparent.d/*.entry /etc/xp2p/nftables/xray-transparent.d/*.entry >/dev/null 2>&1 || true"
            )
        helpers.cleanup_server_install(server_host, server_runner)
        helpers.cleanup_client_install(client_host, client_runner)
        helpers.remove_path(server_host, SERVER_HEARTBEAT_STATE_FILE)
        helpers.remove_path(client_host, CLIENT_HEARTBEAT_STATE_FILE)

    cleanup()
    openwrt_env.install_ipk_on_host(server_host, xp2p_openwrt_ipk)
    openwrt_env.install_ipk_on_host(client_host, xp2p_openwrt_ipk)
    for host in (server_host, client_host):
        openwrt_env._stop_xp2p_services(host)
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
        _set_mode(server_runner, "server", helpers.SERVER_CONFIG_DIR_NAME, "proxy")
        _apply_pending_config_wait(
            server_host,
            "server",
            server_install_path,
            helpers.SERVER_CONFIG_DIR_NAME,
        )
        _wait_for_live_config(server_host, "server")
        server_state = helpers.read_live_server_config(server_host)
        server_routing = helpers.read_live_json(server_host, helpers.SERVER_CONFIG_DIR / "routing.json")
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
        _set_mode(client_runner, "client", helpers.CLIENT_CONFIG_DIR_NAME, "proxy")
        _apply_pending_config_wait(
            client_host,
            "client",
            helpers.INSTALL_ROOT.as_posix(),
            helpers.CLIENT_CONFIG_DIR_NAME,
        )
        _wait_for_live_config(client_host, "client")
        client_state = helpers.read_live_client_config(client_host)
        client_routing = helpers.read_live_json(client_host, helpers.CLIENT_CONFIG_DIR / "routing.json")
        endpoint_tag = helpers.expected_proxy_tag(SERVER_IP)
        helpers.assert_client_reverse_artifacts(client_routing, reverse_tag, endpoint_tag)
        helpers.assert_client_reverse_state(
            client_state,
            reverse_tag,
            endpoint_tag=endpoint_tag,
            user=credential["user"],
            host=SERVER_IP,
        )

        helpers.assert_reverse_cli_output_live(
            server_runner,
            "server",
            server_install_path,
            helpers.SERVER_CONFIG_DIR_NAME,
            reverse_tag,
        )
        helpers.assert_reverse_cli_output_live(
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
    heartbeat_timeout = 30.0
    heartbeat_interval = 2.0
    _ensure_service_running(env["server_host"], "server")
    _ensure_service_running(env["client_host"], "client")
    helpers.wait_for_apply_request_clear(env["server_host"], timeout_seconds=60.0)
    helpers.wait_for_apply_request_clear(env["client_host"], timeout_seconds=60.0)
    helpers.wait_for_live_config(env["server_host"], "server")
    helpers.wait_for_live_config(env["client_host"], "client")
    _wait_for_xray_configs(env["client_host"], helpers.CLIENT_CONFIG_DIR)
    _wait_for_xray_configs(env["server_host"], helpers.SERVER_CONFIG_DIR)
    _dump_run_logs(env["server_host"], "server", "before")
    _dump_run_logs(env["client_host"], "client", "before")
    time.sleep(2.0)
    _dump_run_logs(env["server_host"], "server", "after-start")
    _dump_run_logs(env["client_host"], "client", "after-start")
    _wait_for_listen_port(env["client_host"], CLIENT_SOCKS_PORT)
    time.sleep(2.0)
    helpers.wait_for_heartbeat_state(
        env["server_host"],
        SERVER_HEARTBEAT_STATE_FILE,
        timeout_seconds=heartbeat_timeout,
        poll_interval=heartbeat_interval,
    )
    helpers.wait_for_heartbeat_state(
        env["client_host"],
        CLIENT_HEARTBEAT_STATE_FILE,
        timeout_seconds=heartbeat_timeout,
        poll_interval=heartbeat_interval,
    )
    try:
        tunnel_common.wait_for_alive_entry(
            env["server_runner"],
            "server",
            env["server_install_path"],
            env["endpoint_tag"],
            SERVER_IP,
            env["client_user"],
            env["client_primary_ip"],
            timeout_seconds=heartbeat_timeout,
            poll_interval=heartbeat_interval,
        )
    except AssertionError:
        state_output = env["server_runner"](
            "server",
            "state",
            "--path",
            env["server_install_path"],
            check=True,
        ).stdout or ""
        rows = tunnel_common.parse_state_rows(state_output)
        assert any(row.get("TAG") == env["endpoint_tag"] for row in rows), "Heartbeat entry missing on server"
    try:
        tunnel_common.wait_for_alive_entry(
            env["client_runner"],
            "client",
            helpers.INSTALL_ROOT.as_posix(),
            env["endpoint_tag"],
            SERVER_IP,
            env["client_user"],
            env["client_primary_ip"],
            timeout_seconds=heartbeat_timeout,
            poll_interval=heartbeat_interval,
        )
    except AssertionError:
        state_output = env["client_runner"](
            "client",
            "state",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            check=True,
        ).stdout or ""
        rows = tunnel_common.parse_state_rows(state_output)
        assert any(row.get("TAG") == env["endpoint_tag"] for row in rows), "Heartbeat entry missing on client"
    yield


def _dump_run_logs(host, role: str, stage: str) -> None:
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


def _dump_apply_state(host, role: str, stage: str) -> None:
    config_dir = helpers.CLIENT_CONFIG_DIR if role == "client" else helpers.SERVER_CONFIG_DIR
    host.run(
        "sh -c "
        + shlex.quote(
            " ; ".join(
                (
                    f"echo '--- {role} apply state ({stage}) ---'",
                    "ls -la /etc/xp2p/.apply 2>/dev/null || echo 'missing /etc/xp2p/.apply'",
                    "ls -la /etc/xp2p/.apply/pending 2>/dev/null || echo 'missing /etc/xp2p/.apply/pending'",
                    f"ls -la {config_dir.as_posix()} 2>/dev/null || echo 'missing {config_dir.as_posix()}'",
                    f"if [ -f /etc/xp2p/xp2p-{role}.toml ]; then echo 'live xp2p-{role}.toml: present'; else echo 'live xp2p-{role}.toml: missing'; fi",
                    "if [ -f /etc/xp2p/.apply/apply.request ]; then echo 'apply.request: present'; else echo 'apply.request: missing'; fi",
                    f"/usr/bin/xp2p {role} state --path /etc/xp2p 2>/dev/null || true",
                    f"if [ -f /var/log/xp2p/{role}/service.log ]; then tail -n 200 /var/log/xp2p/{role}/service.log; else echo '/var/log/xp2p/{role}/service.log missing'; fi",
                )
            )
        )
    )


def _wait_for_listen_port(host, port: int, *, timeout_seconds: float = 20.0, interval: float = 1.0) -> None:
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        result = host.run(f"netstat -ltn 2>/dev/null | grep ':{port} '")
        if result.rc == 0:
            return
        time.sleep(interval)
    pytest.fail(f"Port {port} did not start listening on {host.backend.hostname} within {timeout_seconds}s")


def _wait_for_ping_ready(
    runner,
    target: str,
    *,
    port: int | None = None,
    tunnel: bool = False,
    proto: str = "tcp",
    timeout_seconds: float = 30.0,
    interval: float = 1.5,
) -> None:
    deadline = time.time() + timeout_seconds
    last = None
    args = ["ping", target, "--count", "1", "--proto", proto]
    if port is not None:
        args.extend(["--port", str(port)])
    if tunnel:
        args.append("--tunnel")
    while time.time() < deadline:
        last = runner(*args, check=False)
        if last.rc == 0:
            return
        time.sleep(interval)
    stdout = getattr(last, "stdout", "") or ""
    stderr = getattr(last, "stderr", "") or ""
    pytest.fail(
        f"xp2p ping did not become ready for {target} within {timeout_seconds}s.\n"
        f"STDOUT:\n{stdout}\nSTDERR:\n{stderr}"
    )


def _assert_server_alias_reachable(host, runner, target_ip: str, port: int) -> None:
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
        _apply_pending_config_wait(
            client_host,
            "client",
            helpers.INSTALL_ROOT.as_posix(),
            helpers.CLIENT_CONFIG_DIR_NAME,
        )
        with _active_tunnel_sessions(env):
            client_state = helpers.read_live_client_config(client_host)
            entry = tunnel_common.forward_entry_for_target(
                client_state.get("forwards") or [], SERVER_IP, SERVER_DIAGNOSTICS_PORT
            )
            listen_port = tunnel_common.listen_port_from_entry(entry)
            assert listen_port == CLIENT_FORWARD_PORT
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
        _apply_pending_config_wait(
            server_host,
            "server",
            env["server_install_path"],
            helpers.SERVER_CONFIG_DIR_NAME,
        )
        with _active_tunnel_sessions(env):
            server_state = helpers.read_live_server_config(server_host)
            entry = tunnel_common.forward_entry_for_target(
                server_state.get("forward_rules") or [], CLIENT_IP, CLIENT_DIAGNOSTICS_PORT
            )
            listen_port = tunnel_common.listen_port_from_entry(entry)
            _wait_for_listen_port(server_host, listen_port)
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

    if not helpers.path_exists_live(env["server_host"], SERVER_HEARTBEAT_STATE_FILE):
        pytest.fail("Server heartbeat state not found after run start")
    if not helpers.path_exists_live(env["client_host"], CLIENT_HEARTBEAT_STATE_FILE):
        pytest.fail("Client heartbeat state not found after run start")

    server_state = helpers.wait_for_heartbeat_state(
        env["server_host"],
        SERVER_HEARTBEAT_STATE_FILE,
        timeout_seconds=20.0,
        poll_interval=2.0,
    )
    server_entry = helpers.assert_heartbeat_entry(
        server_state,
        expected_tag,
        host=SERVER_IP,
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
            SERVER_IP,
            expected_user,
            expected_client_ip,
            timeout_seconds=20.0,
            poll_interval=2.0,
        )
    except AssertionError:
        state_output = env["server_runner"]("server", "state", "--path", server_install_path, check=True).stdout or ""
        raise AssertionError(
            f"server heartbeat entry not found for {expected_tag}@{SERVER_IP}.\nState output:\n{state_output}"
        ) from None

    try:
        tunnel_common.wait_for_alive_entry(
            env["client_runner"],
            "client",
            client_install_path,
            expected_tag,
            SERVER_IP,
            expected_user,
            expected_client_ip,
            timeout_seconds=20.0,
            poll_interval=2.0,
        )
    except AssertionError:
        state_output = env["client_runner"]("client", "state", "--path", client_install_path, check=True).stdout or ""
        raise AssertionError(
            f"client heartbeat entry not found for {expected_tag}@{SERVER_IP}.\nState output:\n{state_output}"
        ) from None




def _run_server_state_watch(env: dict, duration_seconds: float = 7.0) -> None:
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
        if tuple(tunnel_common.split_state_line(raw.strip())) == tunnel_common.STATE_TABLE_HEADER
    )
    assert header_count >= 2, "xp2p server state --watch did not refresh multiple times"
    assert header_count <= 5, "xp2p server state --watch produced unexpected amount of output"


def test_forward_tunnel_operational(tunnel_environment):
    client_runner = tunnel_environment["client_runner"]

    with _active_tunnel_sessions(tunnel_environment):
        ping_result = client_runner(
            "ping",
            SERVER_IP,
            "--tunnel",
            "--count",
            "3",
            check=True,
        )
        tunnel_common.assert_zero_loss(ping_result, "through SOCKS tunnel")
        _verify_heartbeat_state(tunnel_environment)
        _run_server_state_watch(tunnel_environment)
    _exercise_client_forward_diagnostics(tunnel_environment)
    _exercise_server_forward_diagnostics(tunnel_environment)


def test_client_redirect_through_server(tunnel_environment):
    client_runner = tunnel_environment["client_runner"]
    client_host = tunnel_environment["client_host"]
    server_host = tunnel_environment["server_host"]
    server_runner = tunnel_environment["server_runner"]
    server_install_path = tunnel_environment["server_install_path"]
    endpoint_tag = tunnel_environment["endpoint_tag"]
    nat_snippet = "/etc/nftables.d/xray-transparent.nft"
    nat_entries = "/etc/nftables.d/xray-transparent.d"
    chain_name = "xray_transparent_prerouting"
    target_alias = "10.0.101.50/32"
    listener_port = SERVER_DIAGNOSTICS_PORT
    expected_server_dokodemo_port: int | None = None
    previous_client_mode = None
    previous_server_mode = None

    def _dokodemo_ports(config: dict) -> list[int]:
        ports: list[int] = []
        for inbound in config.get("inbounds", []) or []:
            if not isinstance(inbound, dict):
                continue
            if inbound.get("protocol") != "dokodemo-door":
                continue
            if inbound.get("settings", {}).get("followRedirect") is not True:
                continue
            port_val = inbound.get("port")
            if isinstance(port_val, int):
                ports.append(port_val)
        return ports

    def _safe(text: str | None) -> str:
        if not text:
            return ""
        return text.encode("ascii", "ignore").decode()

    live_list = openwrt_env.run_xp2p_live(
        client_host,
        "client",
        "redirect",
        "list",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
    )
    print("client redirect list --live (pre-add):")
    print((live_list.stdout or "").strip())
    print((live_list.stderr or "").strip())

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
    _dump_apply_state(client_host, "client", "after redirect add (before apply)")
    _apply_pending_config_wait(
        client_host,
        "client",
        helpers.INSTALL_ROOT.as_posix(),
        helpers.CLIENT_CONFIG_DIR_NAME,
    )
    _wait_for_live_config(client_host, "client")
    _dump_apply_state(client_host, "client", "after redirect apply")
    redirect_added = True
    server_redirect_added = False
    server_nat_added = False
    nat_added = False
    server_redirect_cidr: str | None = None
    try:
        previous_server_mode = _ensure_mode(
            server_host, server_runner, "server", helpers.SERVER_CONFIG_DIR_NAME, "proxy"
        )
        previous_client_mode = _ensure_mode(
            client_host, client_runner, "client", helpers.CLIENT_CONFIG_DIR_NAME, "proxy"
        )
        _apply_pending_config_wait(
            server_host,
            "server",
            server_install_path,
            helpers.SERVER_CONFIG_DIR_NAME,
        )
        _wait_for_live_config(server_host, "server")
        _apply_pending_config_wait(
            client_host,
            "client",
            helpers.INSTALL_ROOT.as_posix(),
            helpers.CLIENT_CONFIG_DIR_NAME,
        )
        _wait_for_live_config(client_host, "client")
        with _active_tunnel_sessions(tunnel_environment):
            client_state = helpers.read_live_client_config(client_host)
            client_routing = helpers.read_live_json(client_host, helpers.CLIENT_CONFIG_DIR / "routing.json")
            helpers.assert_redirect_rule(client_routing, CLIENT_REDIRECT_CIDR, endpoint_tag)
            helpers.assert_client_reverse_state(
                client_state,
                tunnel_environment["reverse_tag"],
                endpoint_tag=endpoint_tag,
                user=tunnel_environment["client_user"],
                host=SERVER_IP,
            )
            client_inbounds = helpers.read_live_json(client_host, helpers.CLIENT_CONFIG_DIR / "inbounds.json")
            client_dokodemo_ports = _dokodemo_ports(client_inbounds)
            assert client_dokodemo_ports, (
                "Expected dokodemo-door with followRedirect in client inbounds.json: "
                f"{client_inbounds}"
            )
            server_inbounds = helpers.read_live_json(server_host, helpers.SERVER_CONFIG_DIR / "inbounds.json")
            server_dokodemo_ports = _dokodemo_ports(server_inbounds)
            assert server_dokodemo_ports, (
                "Expected dokodemo-door with followRedirect in server inbounds.json: "
                f"{server_inbounds}"
            )

        plan_output = client_runner(
            "nat-redirect",
            "add",
            "--cidr",
            CLIENT_REDIRECT_CIDR,
            "--print-only",
            "--quiet",
            check=True,
        ).stdout or ""
        assert nat_snippet in plan_output
        assert nat_entries in plan_output
        client_runner(
            "nat-redirect",
            "add",
            "--cidr",
            CLIENT_REDIRECT_CIDR,
            "--quiet",
            check=True,
        )
        nat_added = True
        _dump_apply_state(client_host, "client", "after nat-redirect add (before apply)")
        _apply_pending_config_wait(
            client_host,
            "client",
            helpers.INSTALL_ROOT.as_posix(),
            helpers.CLIENT_CONFIG_DIR_NAME,
        )
        _wait_for_live_config(client_host, "client")
        _dump_apply_state(client_host, "client", "after nat-redirect apply")
        time.sleep(2.0)
        nat_list = client_runner("nat-redirect", "list", check=True).stdout or ""
        assert CLIENT_REDIRECT_CIDR in nat_list

        with _ip_alias(server_host, target_alias):
            with _active_tunnel_sessions(tunnel_environment):
                # baseline: socks ping should work via xray
                target_ip = target_alias.split("/")[0]
                _wait_for_listen_port(server_host, listener_port)
                _assert_server_alias_reachable(server_host, server_runner, target_ip, listener_port)
                server_nat_dump = server_runner("nat-redirect", "list", check=True).stdout or ""
                client_nat_dump = client_runner("nat-redirect", "list", check=True).stdout or ""
                server_chain = server_host.run("nft list chain inet fw4 xray_transparent_prerouting")
                client_chain = client_host.run("nft list chain inet fw4 xray_transparent_prerouting")
                client_socks_netstat = client_host.run("netstat -lpn 2>/dev/null | grep ':51180' || true")
                client_processes = client_host.run("ps w | grep -E 'xp2p|xray' | grep -v grep")
                _dump_apply_state(client_host, "client", "before socks ping")
                _dump_apply_state(server_host, "server", "before socks ping")
                _wait_for_ping_ready(
                    client_runner,
                    target_ip,
                    proto="tcp",
                    tunnel=True,
                    timeout_seconds=30.0,
                )
                socks_ping = client_runner(
                    "ping",
                    target_ip,
                    "--count",
                    "3",
                    "--proto",
                    "tcp",
                    "--tunnel",
                    check=True,
                )
                tunnel_common.assert_zero_loss(
                    socks_ping,
                    f"through SOCKS tunnel before redirect ping. "
                    f"server_nat:\n{_safe(server_nat_dump)}\nclient_nat:\n{_safe(client_nat_dump)}\n"
                    f"server_chain:\n{_safe(server_chain.stdout)}\n{_safe(server_chain.stderr)}\n"
                    f"client_chain:\n{_safe(client_chain.stdout)}\n{_safe(client_chain.stderr)}\n"
                    f"socks_netstat:\n{_safe(client_socks_netstat.stdout)}\n{_safe(client_socks_netstat.stderr)}\n"
                    f"client_ps:\n{_safe(client_processes.stdout)}\n{_safe(client_processes.stderr)}\n",
                )
                _verify_heartbeat_state(tunnel_environment)
            with _active_tunnel_sessions(tunnel_environment):
                _wait_for_ping_ready(client_runner, target_ip, proto="tcp", timeout_seconds=30.0)
                ping_result = client_runner(
                    "ping",
                    target_ip,
                    "--count",
                    "3",
                    "--proto",
                    "tcp",
                    check=True,
                )
                tunnel_common.assert_zero_loss(
                    ping_result,
                    f"while redirecting through server. "
                    f"server_nat:\n{_safe(server_nat_dump)}\nclient_nat:\n{_safe(client_nat_dump)}\n"
                    f"server_chain:\n{_safe(server_chain.stdout)}\n{_safe(server_chain.stderr)}\n"
                    f"client_chain:\n{_safe(client_chain.stdout)}\n{_safe(client_chain.stderr)}\n",
                )

        # Server-side nat-redirect sanity (separate CIDR to avoid client loopback)
        server_redirect_cidr = SERVER_REDIRECT_CIDR
        server_runner(
            "server",
            "redirect",
            "add",
            "--path",
            server_install_path,
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--cidr",
            server_redirect_cidr,
            "--tag",
            tunnel_environment["reverse_tag"],
            check=True,
        )
        server_redirect_added = True
        _dump_apply_state(server_host, "server", "after server redirect add (before apply)")
        _apply_pending_config_wait(
            server_host,
            "server",
            server_install_path,
            helpers.SERVER_CONFIG_DIR_NAME,
        )
        _dump_apply_state(server_host, "server", "after server redirect apply")
        server_runner(
            "nat-redirect",
            "add",
            "--cidr",
            server_redirect_cidr,
            "--quiet",
            check=True,
        )
        server_nat_added = True
        _dump_apply_state(server_host, "server", "after server nat-redirect add (before apply)")
        _apply_pending_config_wait(
            server_host,
            "server",
            server_install_path,
            helpers.SERVER_CONFIG_DIR_NAME,
        )
        _dump_apply_state(server_host, "server", "after server nat-redirect apply")
        time.sleep(2.0)
        server_nat_list = server_runner("nat-redirect", "list", check=True).stdout or ""
        assert server_redirect_cidr in server_nat_list
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
                check=False,
            )
        if nat_added:
            client_runner(
                "nat-redirect",
                "remove",
                "--all",
                check=False,
            )
        if server_nat_added:
            server_runner(
                "nat-redirect",
                "remove",
                "--all",
                check=False,
            )
        if server_redirect_added:
            server_runner(
                "server",
                "redirect",
                "remove",
                "--path",
                server_install_path,
                "--config-dir",
                helpers.SERVER_CONFIG_DIR_NAME,
                "--cidr",
                server_redirect_cidr or CLIENT_REDIRECT_CIDR,
                "--tag",
                tunnel_environment["reverse_tag"],
                check=False,
            )
        _apply_pending_config_wait(
            client_host,
            "client",
            helpers.INSTALL_ROOT.as_posix(),
            helpers.CLIENT_CONFIG_DIR_NAME,
        )
        final_list = openwrt_env.run_xp2p_live(
            client_host,
            "client",
            "redirect",
            "list",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
        ).stdout or ""
        assert CLIENT_REDIRECT_CIDR not in final_list
        if previous_server_mode and previous_server_mode != "proxy":
            _set_mode(server_runner, "server", helpers.SERVER_CONFIG_DIR_NAME, previous_server_mode)
            _apply_pending_config_wait(
                server_host,
                "server",
                server_install_path,
                helpers.SERVER_CONFIG_DIR_NAME,
            )
        if previous_client_mode and previous_client_mode != "proxy":
            _set_mode(client_runner, "client", helpers.CLIENT_CONFIG_DIR_NAME, previous_client_mode)
            _apply_pending_config_wait(
                client_host,
                "client",
                helpers.INSTALL_ROOT.as_posix(),
                helpers.CLIENT_CONFIG_DIR_NAME,
            )


def test_reverse_redirect_via_server_portal(tunnel_environment):
    server_runner = tunnel_environment["server_runner"]
    server_install_path = tunnel_environment["server_install_path"]
    reverse_tag = tunnel_environment["reverse_tag"]
    client_host = tunnel_environment["client_host"]
    server_host = tunnel_environment["server_host"]
    previous_server_mode = None

    alias_cidr = f"{CLIENT_REVERSE_TEST_IP}/32"
    with _ip_alias(client_host, alias_cidr):
        previous_server_mode = _ensure_mode(
            server_host, server_runner, "server", helpers.SERVER_CONFIG_DIR_NAME, "proxy"
        )
        _apply_pending_config_wait(
            server_host,
            "server",
            server_install_path,
            helpers.SERVER_CONFIG_DIR_NAME,
        )
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
        _dump_apply_state(server_host, "server", "after reverse redirect add (before apply)")
        _apply_pending_config_wait(
            server_host,
            "server",
            server_install_path,
            helpers.SERVER_CONFIG_DIR_NAME,
        )
        _dump_apply_state(server_host, "server", "after reverse redirect apply")
        forward_added = False
        try:
            list_output = openwrt_env.run_xp2p_live(
                server_host,
                "server",
                "redirect",
                "list",
                "--path",
                server_install_path,
                "--config-dir",
                helpers.SERVER_CONFIG_DIR_NAME,
            ).stdout or ""
            assert alias_cidr in list_output, f"Server redirect list missing {alias_cidr}"
            with _active_tunnel_sessions(tunnel_environment):
                server_state = helpers.read_live_server_config(server_host)
                server_routing = helpers.read_live_json(server_host, helpers.SERVER_CONFIG_DIR / "routing.json")
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
                f"{CLIENT_REVERSE_TEST_IP}:{CLIENT_DIAGNOSTICS_PORT}",
                "--listen",
                "127.0.0.1",
                "--listen-port",
                str(SERVER_FORWARD_PORT),
                "--proto",
                "tcp",
                check=True,
            )
            forward_added = True
            _dump_apply_state(server_host, "server", "after forward add (before apply)")
            _apply_pending_config_wait(
                server_host,
                "server",
                server_install_path,
                helpers.SERVER_CONFIG_DIR_NAME,
            )
            _dump_apply_state(server_host, "server", "after forward apply")

            server_state = helpers.read_live_server_config(server_host)
            entry = tunnel_common.forward_entry_for_target(
                server_state.get("forward_rules") or [], CLIENT_REVERSE_TEST_IP, CLIENT_DIAGNOSTICS_PORT
            )
            listen_port = tunnel_common.listen_port_from_entry(entry)
            assert listen_port == SERVER_FORWARD_PORT

            ping_result = None
            last_error: BaseException | None = None
            for attempt in range(2):
                with _active_tunnel_sessions(tunnel_environment):
                    _wait_for_listen_port(server_host, SERVER_FORWARD_PORT)
                    _wait_for_listen_port(client_host, CLIENT_DIAGNOSTICS_PORT)
                    time.sleep(2.5)
                    try:
                        _wait_for_ping_ready(
                            server_runner,
                            "127.0.0.1",
                            port=SERVER_FORWARD_PORT,
                            proto="tcp",
                            timeout_seconds=60.0,
                        )
                        ping_result = server_runner(
                            "ping",
                            "127.0.0.1",
                            "--port",
                            str(SERVER_FORWARD_PORT),
                            "--count",
                            "3",
                            "--proto",
                            "tcp",
                            check=True,
                        )
                        tunnel_common.assert_zero_loss(
                            ping_result, f"via server forward targeting {CLIENT_REVERSE_TEST_IP}"
                        )
                        _verify_heartbeat_state(tunnel_environment)
                        last_error = None
                        break
                    except BaseException as exc:
                        if isinstance(exc, (KeyboardInterrupt, SystemExit)):
                            raise
                        helpers.dump_failure_state(server_host, "reverse forward ping failure (server)")
                        helpers.dump_failure_state(client_host, "reverse forward ping failure (client)")
                        last_error = exc
                if attempt == 0:
                    openwrt_env._stop_xp2p_services(server_host)
                    openwrt_env._stop_xp2p_services(client_host)
                    _ensure_service_running(server_host, "server")
                    _ensure_service_running(client_host, "client")
            if last_error is not None:
                raise last_error
        finally:
            if forward_added:
                _server_forward_cmd(
                    tunnel_environment,
                    "remove",
                    "--listen-port",
                    str(SERVER_FORWARD_PORT),
                    check=False,
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
            _apply_pending_config_wait(
                server_host,
                "server",
                server_install_path,
                helpers.SERVER_CONFIG_DIR_NAME,
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
            if previous_server_mode and previous_server_mode != "proxy":
                _set_mode(server_runner, "server", helpers.SERVER_CONFIG_DIR_NAME, previous_server_mode)
                _apply_pending_config_wait(
                    server_host,
                    "server",
                    server_install_path,
                    helpers.SERVER_CONFIG_DIR_NAME,
                )
