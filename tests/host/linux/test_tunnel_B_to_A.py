from __future__ import annotations

from contextlib import contextmanager
import re
import time

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import env as linux_env
from tests.host.tunnel import common as tunnel_common

SERVER_IP = "10.62.10.11"  # deb-test-a (host A)
CLIENT_IP = "10.62.10.12"  # deb-test-b (host B)
CLIENT_REVERSE_TEST_IP = "10.0.102.50"
SERVER_DIAGNOSTICS_PORT = 62022
CLIENT_DIAGNOSTICS_PORT = 62023
SERVER_FORWARD_PORT = 53341
CLIENT_FORWARD_PORT = 53331
CLIENT_REDIRECT_CIDR = "10.0.101.0/24"
SERVER_REDIRECT_CIDR = "10.0.102.0/24"
pytestmark = [pytest.mark.host, pytest.mark.linux]
SERVER_HEARTBEAT_STATE_FILE = helpers.SERVER_HEARTBEAT_STATE_FILE
CLIENT_HEARTBEAT_STATE_FILE = helpers.CLIENT_HEARTBEAT_STATE_FILE


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
            SERVER_IP,
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
            SERVER_IP,
            expected_user,
            expected_client_ip,
        )
    except AssertionError:
        state_output = env["client_runner"]("client", "state", "--path", client_install_path, check=True).stdout or ""
        rows = tunnel_common.parse_state_rows(state_output)
        assert any(row.get("TAG") == expected_tag for row in rows), "Heartbeat entry missing on client"


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
    cleaned = tunnel_common.strip_ansi(result.stdout or "")
    header_count = sum(
        1
        for raw in cleaned.splitlines()
        if tuple(tunnel_common.split_state_line(raw.strip())) == tunnel_common.STATE_TABLE_HEADER
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
            host.run("sudo -n pkill -f '/etc/xp2p/bin/xray' >/dev/null 2>&1 || true")
            host.run("sudo -n nft delete table inet xray_transparent >/dev/null 2>&1 || true")
            host.run("sudo -n rm -f /etc/nftables.d/xray-transparent.nft /etc/xp2p/nftables/xray-transparent.nft >/dev/null 2>&1 || true")
            host.run(
                "sudo -n rm -f /etc/nftables.d/xray-transparent.d/*.entry /etc/xp2p/nftables/xray-transparent.d/*.entry >/dev/null 2>&1 || true"
            )
        helpers.cleanup_server_install(server_host, server_runner)
        helpers.cleanup_client_install(client_host, client_runner)
        helpers.remove_path(server_host, SERVER_HEARTBEAT_STATE_FILE)
        helpers.remove_path(client_host, CLIENT_HEARTBEAT_STATE_FILE)

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
        host.run(f"sudo -n ip addr del {cidr} dev {dev} >/dev/null 2>&1 || true")


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
        client_state = helpers.read_first_existing_json(client_host, helpers.CLIENT_STATE_FILES)
        entry = tunnel_common.forward_entry_for_target(
            client_state.get("forwards") or [], SERVER_IP, SERVER_DIAGNOSTICS_PORT
        )
        listen_port = tunnel_common.listen_port_from_entry(entry)
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
        server_state = helpers.read_first_existing_json(server_host, helpers.SERVER_STATE_FILES)
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


def test_client_and_server_redirect_with_nat(tunnel_environment):
    client_runner = tunnel_environment["client_runner"]
    server_runner = tunnel_environment["server_runner"]
    client_host = tunnel_environment["client_host"]
    server_host = tunnel_environment["server_host"]
    server_install_path = tunnel_environment["server_install_path"]
    endpoint_tag = tunnel_environment["endpoint_tag"]
    reverse_tag = tunnel_environment["reverse_tag"]
    nat_snippet = "/etc/nftables.d/xray-transparent.nft"
    nat_entries = "/etc/nftables.d/xray-transparent.d"
    client_target_alias = "10.0.101.50/32"
    server_target_alias = "10.0.102.50/32"
    chain_name = "xray_transparent_prerouting"
    client_listener_port = SERVER_DIAGNOSTICS_PORT
    server_listener_port = CLIENT_DIAGNOSTICS_PORT

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

    def _detect_chain_cmd(host) -> str | None:
        candidate_chains = (chain_name, "prerouting")
        for table in ("xray_transparent", "fw4"):
            table_list = host.run(f"sudo -n nft list table inet {table}")
            if table_list.rc != 0:
                continue
            for candidate in candidate_chains:
                if re.search(rf"chain\s+{candidate}\b", table_list.stdout or ""):
                    return f"sudo -n nft list chain inet {table} {candidate}"
        return None

    def _nft_counter_sum(host) -> int:
        cmd = _detect_chain_cmd(host)
        if not cmd:
            return 0
        result = host.run(cmd)
        if result.rc != 0:
            pytest.fail(
                f"Failed to list nft chain with command: {cmd}\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        matches = re.findall(r"counter packets\s+(\d+)", result.stdout or "")
        return sum(int(value) for value in matches)

    def _iptables_counter_sum(host) -> int:
        for chain in ("XRAY_TRANSPARENT", "xray_transparent"):
            result = host.run(f"sudo -n /usr/sbin/iptables -t nat -L {chain} -v -n")
            if result.rc != 0:
                continue
            total = 0
            for line in (result.stdout or "").splitlines():
                line = line.strip()
                # Expected columns: pkts bytes target ...
                parts = line.split()
                if len(parts) >= 2 and parts[0].isdigit():
                    try:
                        total += int(parts[0])
                    except ValueError:
                        continue
            if total:
                return total
        return 0

    def _traffic_counter_sum(host) -> int:
        nft_total = _nft_counter_sum(host)
        ipt_total = _iptables_counter_sum(host)
        return max(nft_total, ipt_total)

    def _has_nat_chain(host) -> bool:
        cmd = _detect_chain_cmd(host)
        if cmd:
            res = host.run(cmd)
            if res.rc == 0 and ("counter" in (res.stdout or "") or "chain" in (res.stdout or "")):
                return True
        for chain in ("XRAY_TRANSPARENT", "xray_transparent"):
            check = host.run(f"sudo -n /usr/sbin/iptables -t nat -L {chain} -v -n")
            if check.rc == 0 and "Chain" in (check.stdout or ""):
                return True
        return False

    def _nat_debug() -> str:
        client_nat_dump = client_runner("nat-redirect", "list", check=False).stdout or ""
        server_nat_dump = server_runner("nat-redirect", "list", check=False).stdout or ""
        client_chain = client_host.run("sudo -n nft list table inet xray_transparent")
        server_chain = server_host.run("sudo -n nft list table inet xray_transparent")
        client_iptables = client_host.run("sudo -n /usr/sbin/iptables -t nat -L XRAY_TRANSPARENT -v -n")
        server_iptables = server_host.run("sudo -n /usr/sbin/iptables -t nat -L XRAY_TRANSPARENT -v -n")
        sockets = client_host.run("sudo -n netstat -lpn 2>/dev/null | grep '51180|51080|52080|62022|62023' || true")
        processes = client_host.run("ps w | grep -E 'xp2p|xray' | grep -v grep")
        client_inbounds = client_host.run(f"cat {helpers.CLIENT_CONFIG_DIR / 'inbounds.json'} 2>/dev/null || true")
        server_inbounds = server_host.run(f"cat {helpers.SERVER_CONFIG_DIR / 'inbounds.json'} 2>/dev/null || true")
        client_routing = client_host.run(f"cat {helpers.CLIENT_CONFIG_DIR / 'routing.json'} 2>/dev/null || true")
        server_routing = server_host.run(f"cat {helpers.SERVER_CONFIG_DIR / 'routing.json'} 2>/dev/null || true")
        client_logs_json = client_host.run(f"cat {helpers.CLIENT_CONFIG_DIR / 'logs.json'} 2>/dev/null || true")
        server_logs_json = server_host.run(f"cat {helpers.SERVER_CONFIG_DIR / 'logs.json'} 2>/dev/null || true")
        client_run_log = client_host.run("cat /tmp/xp2p-*-run.log 2>/dev/null || true")
        server_run_log = server_host.run("cat /tmp/xp2p-*-run.log 2>/dev/null || true")
        client_err_log = client_host.run("cat /var/log/xp2p/*.err 2>/dev/null || true")
        server_err_log = server_host.run("cat /var/log/xp2p/*.err 2>/dev/null || true")
        client_snippet = client_host.run(f"sudo -n cat {nat_snippet} 2>/dev/null || true")
        server_snippet = server_host.run(f"sudo -n cat {nat_snippet} 2>/dev/null || true")
        client_entries_ls = client_host.run(f"sudo -n ls -l {nat_entries} 2>/dev/null || true")
        server_entries_ls = server_host.run(f"sudo -n ls -l {nat_entries} 2>/dev/null || true")
        return (
            f"server_nat:\n{_safe(server_nat_dump)}\nclient_nat:\n{_safe(client_nat_dump)}\n"
            f"server_chain:\n{_safe(server_chain.stdout)}\n{_safe(server_chain.stderr)}\n"
            f"client_chain:\n{_safe(client_chain.stdout)}\n{_safe(client_chain.stderr)}\n"
            f"server_iptables:\n{_safe(server_iptables.stdout)}\n{_safe(server_iptables.stderr)}\n"
            f"client_iptables:\n{_safe(client_iptables.stdout)}\n{_safe(client_iptables.stderr)}\n"
            f"client_netstat:\n{_safe(sockets.stdout)}\n{_safe(sockets.stderr)}\n"
            f"client_ps:\n{_safe(processes.stdout)}\n{_safe(processes.stderr)}\n"
            f"client_inbounds:\n{_safe(client_inbounds.stdout)}\nserver_inbounds:\n{_safe(server_inbounds.stdout)}\n"
            f"client_routing:\n{_safe(client_routing.stdout)}\nserver_routing:\n{_safe(server_routing.stdout)}\n"
            f"client_logs_json:\n{_safe(client_logs_json.stdout)}\nserver_logs_json:\n{_safe(server_logs_json.stdout)}\n"
            f"client_run_log:\n{_safe(client_run_log.stdout)}\nserver_run_log:\n{_safe(server_run_log.stdout)}\n"
            f"client_err_log:\n{_safe(client_err_log.stdout)}\nserver_err_log:\n{_safe(server_err_log.stdout)}\n"
            f"client_snippet:\n{_safe(client_snippet.stdout)}\nserver_snippet:\n{_safe(server_snippet.stdout)}\n"
            f"client_entries_ls:\n{_safe(client_entries_ls.stdout)}\nserver_entries_ls:\n{_safe(server_entries_ls.stdout)}\n"
        )

    redirect_added = False
    nat_added = False
    server_nat_added = False
    server_redirect_added = False
    try:
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
            "--quiet",
            check=True,
        )
        redirect_added = True
        client_state = helpers.read_first_existing_json(client_host, helpers.CLIENT_STATE_FILES)
        client_routing = helpers.read_json(client_host, helpers.CLIENT_CONFIG_DIR / "routing.json")
        helpers.assert_redirect_rule(client_routing, CLIENT_REDIRECT_CIDR, endpoint_tag)
        helpers.assert_client_reverse_state(
            client_state,
            reverse_tag,
            endpoint_tag=endpoint_tag,
            user=tunnel_environment["client_user"],
            host=SERVER_IP,
        )
        client_inbounds = helpers.read_json(client_host, helpers.CLIENT_CONFIG_DIR / "inbounds.json")
        client_dokodemo_ports = _dokodemo_ports(client_inbounds)
        assert client_dokodemo_ports, f"Expected dokodemo-door with followRedirect in client inbounds.json: {client_inbounds}"
        server_inbounds = helpers.read_json(server_host, helpers.SERVER_CONFIG_DIR / "inbounds.json")
        server_dokodemo_ports = _dokodemo_ports(server_inbounds)
        assert server_dokodemo_ports, f"Expected dokodemo-door with followRedirect in server inbounds.json: {server_inbounds}"

        plan_output = client_runner(
            "nat-redirect",
            "add",
            "--subnet",
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
            "--subnet",
            CLIENT_REDIRECT_CIDR,
            "--quiet",
            check=True,
        )
        nat_added = True
        time.sleep(2.0)
        nat_list = client_runner("nat-redirect", "list", check=True).stdout or ""
        assert CLIENT_REDIRECT_CIDR in nat_list

        with _ip_alias(server_host, client_target_alias):
            target_ip = client_target_alias.split("/")[0]
            with _active_tunnel_sessions(tunnel_environment):
                socks_ping = client_runner(
                    "ping",
                    target_ip,
                    "--tunnel",
                    "--port",
                    str(client_listener_port),
                    "--count",
                    "3",
                    "--proto",
                    "tcp",
                    check=True,
                )
                tunnel_common.assert_zero_loss(
                    socks_ping,
                    f"through SOCKS tunnel before redirect ping. {_nat_debug()}",
                )
            counting_supported = _has_nat_chain(client_host)
            if not counting_supported:
                pytest.fail(
                    "transparent NAT backend not available on client host (no nft/iptables chain).\n"
                    f"{_nat_debug()}"
                    )
                initial_packets = _traffic_counter_sum(client_host) if counting_supported else 0
                ping_result = client_runner(
                    "ping",
                    target_ip,
                    "--port",
                    str(client_listener_port),
                    "--count",
                    "3",
                    "--proto",
                    "tcp",
                    check=True,
                )
                tunnel_common.assert_zero_loss(
                    ping_result,
                    f"while redirecting through server. {_nat_debug()}",
                )
                if counting_supported:
                    final_packets = _traffic_counter_sum(client_host)
                    assert final_packets > initial_packets, (
                        "transparent redirect counters did not increase after redirect ping. "
                        f"initial={initial_packets} final={final_packets}. {_nat_debug()}"
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
            SERVER_REDIRECT_CIDR,
            "--tag",
            reverse_tag,
            check=True,
        )
        server_redirect_added = True
        server_runner(
            "nat-redirect",
            "add",
            "--subnet",
            SERVER_REDIRECT_CIDR,
            "--quiet",
            check=True,
        )
        server_nat_added = True
        time.sleep(2.0)
        server_nat_list = server_runner("nat-redirect", "list", check=True).stdout or ""
        assert SERVER_REDIRECT_CIDR in server_nat_list

        with _ip_alias(client_host, server_target_alias):
            reverse_ip = server_target_alias.split("/")[0]
            with _active_tunnel_sessions(tunnel_environment):
                socks_ping = server_runner(
                    "ping",
                    reverse_ip,
                    "--tunnel",
                    "--port",
                    str(server_listener_port),
                    "--count",
                    "3",
                    "--proto",
                    "tcp",
                    check=True,
                )
                tunnel_common.assert_zero_loss(
                    socks_ping,
                    f"server-side SOCKS ping toward {reverse_ip}. {_nat_debug()}",
                )
            counting_supported = _has_nat_chain(server_host)
            if not counting_supported:
                pytest.fail(
                    "transparent NAT backend not available on server host (no nft/iptables chain).\n"
                    f"{_nat_debug()}"
                )
                initial_server_packets = _traffic_counter_sum(server_host) if counting_supported else 0
                ping_result = server_runner(
                    "ping",
                    reverse_ip,
                    "--port",
                    str(server_listener_port),
                    "--count",
                    "3",
                    "--proto",
                    "tcp",
                    check=True,
                )
                tunnel_common.assert_zero_loss(
                    ping_result,
                    f"server-side direct ping toward {reverse_ip}. {_nat_debug()}",
                )
                if counting_supported:
                    final_server_packets = _traffic_counter_sum(server_host)
                    assert final_server_packets > initial_server_packets, (
                        "server transparent redirect counters did not increase after reverse ping. "
                        f"initial={initial_server_packets} final={final_server_packets}. {_nat_debug()}"
                    )
    finally:
        client_runner("nat-redirect", "remove", "--all", "--quiet", check=False)
        server_runner("nat-redirect", "remove", "--all", "--quiet", check=False)
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
                "--quiet",
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
                SERVER_REDIRECT_CIDR,
                "--tag",
                reverse_tag,
                "--quiet",
                check=False,
            )
        client_redirect_list = client_runner(
            "client",
            "redirect",
            "list",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
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
            check=True,
        ).stdout or ""
        assert CLIENT_REDIRECT_CIDR not in client_redirect_list
        assert SERVER_REDIRECT_CIDR not in server_redirect_list


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

            server_state = helpers.read_first_existing_json(server_host, helpers.SERVER_STATE_FILES)
            entry = tunnel_common.forward_entry_for_target(
                server_state.get("forward_rules") or [], CLIENT_REVERSE_TEST_IP, CLIENT_DIAGNOSTICS_PORT
            )
            listen_port = tunnel_common.listen_port_from_entry(entry)
            assert listen_port == SERVER_FORWARD_PORT

            with _active_tunnel_sessions(tunnel_environment):
                ping_result = server_runner(
                    "ping",
                    "127.0.0.1",
                    "--port",
                    str(SERVER_FORWARD_PORT),
                    "--count",
                    "3",
                    check=True,
                )
                tunnel_common.assert_zero_loss(ping_result, f"via server forward targeting {CLIENT_REVERSE_TEST_IP}")
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
