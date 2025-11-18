from __future__ import annotations

from contextlib import contextmanager
import re
import time

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import env as linux_env

SERVER_IP = "10.62.10.11"  # deb-test-a (host A)
CLIENT_IP = "10.62.10.12"  # deb-test-b (host B)
CLIENT_REVERSE_TEST_IP = "10.62.20.5"
DIAGNOSTICS_PORT = 62022
SERVER_FORWARD_PORT = 53341
CLIENT_REDIRECT_CIDR = "10.200.50.0/24"
pytestmark = [pytest.mark.host, pytest.mark.linux]
HEARTBEAT_STATE_FILE = helpers.INSTALL_ROOT / "state-heartbeat.json"
STATE_TABLE_HEADER = (
    "TAG",
    "HOST",
    "STATUS",
    "LAST_RTT",
    "AVG_RTT",
    "LAST_UPDATE",
    "CLIENT_USER",
    "CLIENT_IP",
)
ANSI_ESCAPE_RE = re.compile(r"\x1b\[[0-9;]*[A-Za-z]")


def _runner(host):
    def _run(*args: str, check: bool = False):
        result = linux_env.run_xp2p(host, *args)
        if check and result.rc != 0:
            pytest.fail(
                "xp2p command failed "
                f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        return result

    return _run


def _forward_entry_for_target(entries: list[dict], target_ip: str, target_port: int) -> dict:
    """Locate a forward entry recorded in install-state for the provided target."""
    normalized_ip = target_ip.strip()
    normalized_port = int(target_port)
    for entry in entries or []:
        if not isinstance(entry, dict):
            continue
        recorded_ip = (entry.get("target_ip") or entry.get("targetIP") or "").strip()
        recorded_port = int(entry.get("target_port") or entry.get("targetPort") or 0)
        if recorded_ip == normalized_ip and recorded_port == normalized_port:
            return entry
    raise AssertionError(f"Forward entry targeting {target_ip}:{target_port} not found in state")


def _listen_port_from_entry(entry: dict) -> int:
    """Extract listen port from the forward entry."""
    port = int(entry.get("listen_port") or entry.get("listenPort") or 0)
    if port <= 0:
        raise AssertionError("Forward entry is missing listen port")
    return port


def _assert_zero_loss(ping_result, context: str) -> None:
    stdout = (ping_result.stdout or "").lower()
    assert "0% loss" in stdout, (
        f"xp2p ping {context} did not report full delivery:\n"
        f"{ping_result.stdout}"
    )


def _ping_with_retries(runner, args: tuple[str, ...], context: str, attempts: int = 3, delay_seconds: float = 2.0):
    last_result = None
    for attempt in range(1, attempts + 1):
        result = runner(*args, check=False)
        if result.rc == 0:
            return result
        last_result = result
        if attempt < attempts:
            time.sleep(delay_seconds)
    assert last_result is not None, "xp2p ping failed but no result captured"
    pytest.fail(
        f"xp2p ping {context} failed after {attempts} attempts "
        f"(exit {last_result.rc}).\nSTDOUT:\n{last_result.stdout}\nSTDERR:\n{last_result.stderr}"
    )


def _detect_primary_ipv4(host) -> str:
    command = "ip -o -4 addr show scope global | awk '{print $4}' | cut -d/ -f1 | head -n1"
    result = host.run(command)
    ip_address = (result.stdout or "").strip()
    if result.rc != 0 or not ip_address:
        pytest.fail(
            "Failed to detect primary IPv4 address on "
            f"{host.backend.hostname}. STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return ip_address


def _wait_for_heartbeat_file(host, path, *, timeout_seconds: float = 45.0, poll_interval: float = 1.5) -> None:
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        if helpers.path_exists(host, path):
            return
        time.sleep(poll_interval)
    pytest.fail(f"Heartbeat state file {path} not found on {host.backend.hostname}")


def _strip_ansi(value: str | None) -> str:
    if not value:
        return ""
    return ANSI_ESCAPE_RE.sub("", value)


def _parse_state_rows(output: str) -> list[dict[str, str]]:
    cleaned = _strip_ansi(output)
    header = None
    rows: list[dict[str, str]] = []
    for raw_line in cleaned.splitlines():
        line = raw_line.strip()
        if not line:
            continue
        cells = _split_state_line(line)
        if not cells:
            continue
        if tuple(cells[: len(STATE_TABLE_HEADER)]) == STATE_TABLE_HEADER:
            header = list(STATE_TABLE_HEADER)
            continue
        if not header:
            continue
        if len(cells) != len(header):
            continue
        if all(cell.strip() == "-" for cell in cells):
            continue
        rows.append({header[idx]: cell.strip() for idx, cell in enumerate(cells)})
    return rows


def _split_state_line(line: str) -> list[str]:
    parts = [segment.strip() for segment in line.split("\t") if segment.strip()]
    if len(parts) >= len(STATE_TABLE_HEADER):
        return parts[: len(STATE_TABLE_HEADER)]
    regex_parts = [segment.strip() for segment in re.split(r"\s{2,}", line) if segment.strip()]
    if len(regex_parts) >= len(STATE_TABLE_HEADER):
        return regex_parts[: len(STATE_TABLE_HEADER)]
    return regex_parts or parts


def _wait_for_alive_entry(
    runner,
    role: str,
    install_path: str,
    expected_tag: str,
    expected_host: str,
    expected_user: str,
    expected_client_ip: str,
    *,
    timeout_seconds: float = 60.0,
    poll_interval: float = 2.0,
) -> dict:
    deadline = time.time() + timeout_seconds
    last_stdout = ""
    while time.time() < deadline:
        result = runner(
            role,
            "state",
            "--path",
            install_path,
            check=True,
        )
        last_stdout = result.stdout or ""
        for row in _parse_state_rows(last_stdout):
            tag = row.get("TAG", "").strip()
            host_value = row.get("HOST", "").strip()
            status = row.get("STATUS", "").strip().lower()
            if tag != expected_tag or host_value != expected_host or status != "alive":
                continue
            client_user = row.get("CLIENT_USER", "").strip()
            client_ip = row.get("CLIENT_IP", "").strip()
            assert (
                client_user == expected_user
            ), f"Heartbeat CLIENT_USER mismatch (expected {expected_user}, got {client_user})"
            assert (
                client_ip == expected_client_ip
            ), f"Heartbeat CLIENT_IP mismatch (expected {expected_client_ip}, got {client_ip})"
            return row
        time.sleep(poll_interval)
    pytest.fail(
        "Alive heartbeat entry not observed for "
        f"{expected_tag}@{expected_host}. Last xp2p {role} state output:\n{last_stdout}"
    )


def _verify_heartbeat_state(env: dict) -> None:
    expected_tag = env["endpoint_tag"]
    expected_user = env["client_user"]
    expected_client_ip = env["client_primary_ip"]
    server_install_path = env["server_install_path"]
    client_install_path = helpers.INSTALL_ROOT.as_posix()

    _wait_for_heartbeat_file(env["server_host"], HEARTBEAT_STATE_FILE)
    _wait_for_heartbeat_file(env["client_host"], HEARTBEAT_STATE_FILE)
    _wait_for_alive_entry(
        env["server_runner"],
        "server",
        server_install_path,
        expected_tag,
        SERVER_IP,
        expected_user,
        expected_client_ip,
    )
    _wait_for_alive_entry(
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
    xp2p_binary = linux_env.INSTALL_PATH.as_posix()
    timeout_arg = f"{duration_seconds:.0f}s"
    command = (
        f"timeout -k 1s {timeout_arg} sudo -n {xp2p_binary} server state "
        f"--watch --interval 2s --path {install_path}"
    )
    result = server_host.run(command)
    if result.rc not in (0, 124):
        pytest.fail(
            "xp2p server state --watch failed "
            f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    cleaned = _strip_ansi(result.stdout or "")
    header_count = sum(
        1
        for raw in cleaned.splitlines()
        if tuple(_split_state_line(raw.strip())) == STATE_TABLE_HEADER
    )
    assert header_count >= 2, "xp2p server state --watch did not refresh multiple times"
    assert header_count <= 5, "xp2p server state --watch produced unexpected amount of output"


@pytest.fixture(scope="module")
def tunnel_environment(linux_host_factory, xp2p_linux_versions):
    server_host = linux_host_factory(linux_env.DEFAULT_CLIENT)
    client_host = linux_host_factory(linux_env.DEFAULT_SERVER)
    server_runner = _runner(server_host)
    client_runner = _runner(client_host)
    server_install_path = helpers.INSTALL_ROOT.as_posix()
    client_primary_ip = _detect_primary_ipv4(client_host)

    def cleanup():
        for host in (server_host, client_host):
            host.run("sudo -n pkill -f '/usr/bin/xp2p server run' >/dev/null 2>&1 || true")
            host.run("sudo -n pkill -f '/usr/bin/xp2p client run' >/dev/null 2>&1 || true")
            host.run("sudo -n pkill -f '/etc/xp2p/bin/xray' >/dev/null 2>&1 || true")
        helpers.cleanup_server_install(server_host, server_runner)
        helpers.cleanup_client_install(client_host, client_runner)
        for host in (server_host, client_host):
            helpers.remove_path(host, HEARTBEAT_STATE_FILE)

    cleanup()
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
        recorded_tags = {tag for tag in client_state.get("reverse", {}).keys()}
        assert recorded_tags == {reverse_tag}, f"Unexpected reverse entries recorded: {recorded_tags}"

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
    with linux_env.xp2p_run_session(
        env["server_host"],
        "server",
        env["server_install_path"],
        helpers.SERVER_CONFIG_DIR_NAME,
        helpers.SERVER_LOG_FILE,
    ), linux_env.xp2p_run_session(
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
        host.run(f"sudo -n ip addr del {cidr} dev {dev}")


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
        entry = _forward_entry_for_target(client_state.get("forwards") or [], SERVER_IP, DIAGNOSTICS_PORT)
        listen_port = _listen_port_from_entry(entry)

        with _active_tunnel_sessions(env):
            ping_result = _ping_with_retries(
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
            _assert_zero_loss(ping_result, f"via client forward on port {listen_port}")
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
    server_install_path = env["server_install_path"]
    forward_target = f"{CLIENT_IP}:{DIAGNOSTICS_PORT}"
    listen_port = None
    redirect_cidr = f"{CLIENT_IP}/32"
    redirect_added = False
    enable_redirect = False
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
        entry = _forward_entry_for_target(server_state.get("forward_rules") or [], CLIENT_IP, DIAGNOSTICS_PORT)
        listen_port = _listen_port_from_entry(entry)

        if enable_redirect:
            server_runner(
                "server",
                "redirect",
                "add",
                "--path",
                server_install_path,
                "--config-dir",
                helpers.SERVER_CONFIG_DIR_NAME,
                "--cidr",
                redirect_cidr,
                "--tag",
                env["reverse_tag"],
                check=True,
            )
            redirect_added = True

        with _active_tunnel_sessions(env):
            ping_result = _ping_with_retries(
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
            _assert_zero_loss(ping_result, f"via server forward on port {listen_port}")
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
        if redirect_added:
            server_runner(
                "server",
                "redirect",
                "remove",
                "--path",
                server_install_path,
                "--config-dir",
                helpers.SERVER_CONFIG_DIR_NAME,
                "--cidr",
                redirect_cidr,
                "--tag",
                env["reverse_tag"],
                check=True,
            )


def test_forward_tunnel_operational(tunnel_environment):
    client_runner = tunnel_environment["client_runner"]

    with _active_tunnel_sessions(tunnel_environment):
        ping_result = _ping_with_retries(
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
        _assert_zero_loss(ping_result, "through SOCKS tunnel")
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
        "--host",
        SERVER_IP,
        check=True,
    )
    try:
        redirect_list = client_runner(
            "client",
            "redirect",
            "list",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            check=True,
        ).stdout or ""
        assert CLIENT_REDIRECT_CIDR in redirect_list

        routing = helpers.read_json(client_host, helpers.CLIENT_CONFIG_DIR / "routing.json")
        helpers.assert_redirect_rule(routing, CLIENT_REDIRECT_CIDR, endpoint_tag)
    finally:
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
            "--host",
            SERVER_IP,
            check=True,
        )
        routing_after = helpers.read_json(client_host, helpers.CLIENT_CONFIG_DIR / "routing.json")
        helpers.assert_no_redirect_rule(routing_after, CLIENT_REDIRECT_CIDR, endpoint_tag)
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
            entry = _forward_entry_for_target(server_state.get("forward_rules") or [], CLIENT_REVERSE_TEST_IP, DIAGNOSTICS_PORT)
            listen_port = _listen_port_from_entry(entry)
            assert listen_port == SERVER_FORWARD_PORT

            with _active_tunnel_sessions(tunnel_environment):
                ping_result = _ping_with_retries(
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
                _assert_zero_loss(ping_result, f"via server forward targeting {CLIENT_REVERSE_TEST_IP}")
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
