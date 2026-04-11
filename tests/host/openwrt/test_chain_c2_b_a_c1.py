from __future__ import annotations

from contextlib import contextmanager
import time

import pytest

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env
from tests.host.tunnel import common as tunnel_common

SERVER_MACHINE = openwrt_env.OPENWRT_MACHINES[0]
CLIENT_MACHINE = openwrt_env.OPENWRT_MACHINES[1]
ALPINE_C1 = openwrt_env.ALPINE_MACHINES[0]
ALPINE_C2 = openwrt_env.ALPINE_MACHINES[1]

SERVER_TUNNEL_IP = "10.63.30.11"
C1_LAN_CIDR = "10.0.101.0/24"
C1_LAN_GATEWAY = "10.0.101.1"
DIAG_LISTEN = "0.0.0.0:62022"
pytestmark = [pytest.mark.host, pytest.mark.linux]


@contextmanager
def _timed(label: str):
    start = time.perf_counter()
    try:
        yield
    finally:
        elapsed = time.perf_counter() - start
        print(f"TIMING: {label}: {elapsed:.2f}s")


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


def _alpine_guest(host, script: str, *args: str):
    result = openwrt_env.run_alpine_guest_script(host, script, *args)
    if result.rc != 0:
        pytest.fail(
            "guest script failed "
            f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return result


def _current_mode(host, role: str) -> str:
    helpers.wait_for_live_config(host, role)
    if role == "server":
        config = helpers.read_live_server_config(host)
    elif role == "client":
        config = helpers.read_live_client_config(host)
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


def _apply_pending_config(host, role: str) -> None:
    pending_path = helpers.CONFIG_PENDING_ROOT / f"xp2p-{role}.toml"
    if not helpers.path_exists_exact(host, pending_path) and not helpers.path_exists_exact(
        host, helpers.APPLY_REQUEST
    ):
        return
    helpers.ensure_service_running(host, role)
    helpers.wait_for_apply_request_clear(host, timeout_seconds=60.0)
    helpers.wait_for_live_config(host, role)


@pytest.fixture(scope="module")
def chain_environment(openwrt_host_factory, xp2p_openwrt_ipk):
    server_host = openwrt_host_factory(SERVER_MACHINE)
    client_host = openwrt_host_factory(CLIENT_MACHINE)
    server_runner = _runner(server_host)
    client_runner = _runner(client_host)
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
        client_runner("nat-redirect", "remove", "--all", check=False)
        helpers.cleanup_server_install(server_host, server_runner)
        helpers.cleanup_client_install(client_host, client_runner)
        helpers.remove_path(server_host, helpers.SERVER_HEARTBEAT_STATE_FILE)
        helpers.remove_path(client_host, helpers.CLIENT_HEARTBEAT_STATE_FILE)

    with _timed("chain_environment cleanup"):
        cleanup()
    with _timed("install ipk (server)"):
        openwrt_env.install_ipk_on_host(server_host, xp2p_openwrt_ipk)
    with _timed("install ipk (client)"):
        openwrt_env.install_ipk_on_host(client_host, xp2p_openwrt_ipk)

    try:
        with _timed("server install"):
            server_install = server_runner(
                "server",
                "install",
                "--path",
                helpers.INSTALL_ROOT.as_posix(),
                "--config-dir",
                helpers.SERVER_CONFIG_DIR_NAME,
                "--host",
                SERVER_TUNNEL_IP,
                "--force",
                check=True,
            )
        credential = helpers.extract_trojan_credential(server_install.stdout or "")
        reverse_tag = helpers.expected_reverse_tag(credential["user"], SERVER_TUNNEL_IP)
        endpoint_tag = helpers.expected_proxy_tag(SERVER_TUNNEL_IP)

        with _timed("client install"):
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
        helpers.wait_for_pending_config(server_host, "server")
        helpers.wait_for_pending_config(client_host, "client")
        _apply_pending_config(server_host, "server")
        _apply_pending_config(client_host, "client")
        helpers.wait_for_live_config(server_host, "server")
        helpers.wait_for_live_config(client_host, "client")
        yield {
            "server_host": server_host,
            "client_host": client_host,
            "server_runner": server_runner,
            "client_runner": client_runner,
            "server_install_path": helpers.INSTALL_ROOT.as_posix(),
            "reverse_tag": reverse_tag,
            "endpoint_tag": endpoint_tag,
            "client_user": credential["user"],
            "client_primary_ip": client_primary_ip,
        }
    finally:
        cleanup()


def test_chain_c2_b_a_c1_redirect_nat(chain_environment, alpine_c1_host, alpine_c2_host):
    test_start = time.perf_counter()
    server_runner = chain_environment["server_runner"]
    client_runner = chain_environment["client_runner"]
    endpoint_tag = chain_environment["endpoint_tag"]
    redirect_added = False
    nat_added = False
    diag_started = False
    client_host = chain_environment["client_host"]
    previous_client_mode = None
    server_host = chain_environment["server_host"]
    previous_server_mode = None
    route_backup: list[str] = []
    blackhole_added = False

    with _timed("guest route setup"):
        _alpine_guest(alpine_c1_host, "scripts/linux/ensure_route.sh", "10.0.102.0/24", C1_LAN_GATEWAY)
        _alpine_guest(alpine_c2_host, "scripts/linux/ensure_route.sh", "10.0.101.0/24", "10.0.102.1")
    with _timed("guest diag start"):
        c1_ip = _alpine_guest(alpine_c1_host, "scripts/linux/get_interface_ip.sh", "eth1").stdout.strip()
        _alpine_guest(alpine_c1_host, "scripts/linux/start_xp2p_diag.sh", DIAG_LISTEN, "tcp")
    diag_started = True

    try:
        with _timed("ensure proxy mode"):
            previous_server_mode = _ensure_mode(
                server_host, server_runner, "server", helpers.SERVER_CONFIG_DIR_NAME, "proxy"
            )
            previous_client_mode = _ensure_mode(
                client_host, client_runner, "client", helpers.CLIENT_CONFIG_DIR_NAME, "proxy"
            )
            _apply_pending_config(server_host, "server")
            _apply_pending_config(client_host, "client")
            helpers.wait_for_live_config(server_host, "server")
            helpers.wait_for_live_config(client_host, "client")
        with _timed("baseline direct ping"):
            initial_ping = server_runner(
                "ping",
                c1_ip,
                "--count",
                "1",
                check=True,
            )
        tunnel_common.assert_zero_loss(initial_ping, f"direct ping to {c1_ip}")

        with _timed("add client redirect"):
            client_runner(
                "client",
                "redirect",
                "add",
                "--path",
                helpers.INSTALL_ROOT.as_posix(),
                "--config-dir",
                helpers.CLIENT_CONFIG_DIR_NAME,
                "--cidr",
                C1_LAN_CIDR,
                "--tag",
                endpoint_tag,
                check=True,
            )
        redirect_added = True

        helpers.ensure_service_running(chain_environment["server_host"], "server")
        helpers.ensure_service_running(chain_environment["client_host"], "client")
        helpers.wait_for_apply_request_clear(chain_environment["server_host"], timeout_seconds=60.0)
        helpers.wait_for_apply_request_clear(chain_environment["client_host"], timeout_seconds=60.0)
        helpers.wait_for_live_config(chain_environment["server_host"], "server")
        helpers.wait_for_live_config(chain_environment["client_host"], "client")
        with _timed("tunnel warmup"):
            time.sleep(2.0)
        with _timed("tunnel ping"):
            tunnel_ping = client_runner(
                "ping",
                SERVER_TUNNEL_IP,
                "--tunnel",
                "--count",
                "1",
                check=True,
            )
        tunnel_common.assert_zero_loss(tunnel_ping, "through SOCKS tunnel")

        with _timed("redirect ping via tunnel"):
            redirect_ping = client_runner(
                "ping",
                C1_LAN_GATEWAY,
                "--tunnel",
                "--count",
                "1",
                check=True,
            )
        tunnel_common.assert_zero_loss(redirect_ping, "redirect via SOCKS tunnel")

        with _timed("prune direct route (if any)"):
            route_result = client_host.run(f"ip route show {C1_LAN_CIDR}")
            if route_result.rc == 0 and (route_result.stdout or "").strip():
                route_backup = [line.strip() for line in route_result.stdout.splitlines() if line.strip()]
                client_host.run(f"ip route del {C1_LAN_CIDR} >/dev/null 2>&1 || true")
            blackhole_result = client_host.run(f"ip route replace blackhole {C1_LAN_CIDR}")
            blackhole_added = blackhole_result.rc == 0

        with _timed("ping without nat-redirect"):
            missing_nat = client_runner(
                "ping",
                C1_LAN_GATEWAY,
                "--count",
                "1",
                check=False,
            )
        if missing_nat.rc == 0:
            route_get = client_host.run(f"ip route get {C1_LAN_GATEWAY}")
            route_tables = client_host.run("ip route show table main")
            rules_output = client_host.run("ip rule show")
            addr_output = client_host.run("ip -o -4 addr show")
            fw4_pre = client_host.run("nft list chain inet fw4 xray_transparent_prerouting")
            fw4_out = client_host.run("nft list chain inet fw4 xray_transparent_output")
            nat_table = client_host.run("nft list table inet xray_transparent")
            client_redirects = client_runner(
                "client",
                "redirect",
                "list",
                "--path",
                helpers.INSTALL_ROOT.as_posix(),
                "--config-dir",
                helpers.CLIENT_CONFIG_DIR_NAME,
                check=False,
            )
            nat_list = client_runner("nat-redirect", "list", check=False)
            pytest.fail(
                "expected ping to fail without nat-redirect.\n"
                f"STDOUT:\n{missing_nat.stdout}\nSTDERR:\n{missing_nat.stderr}"
                f"\nRoute:\n{route_result.stdout}\n{route_result.stderr}"
                f"\nRoute get:\n{route_get.stdout}\n{route_get.stderr}"
                f"\nRoutes (main table):\n{route_tables.stdout}\n{route_tables.stderr}"
                f"\nRules:\n{rules_output.stdout}\n{rules_output.stderr}"
                f"\nAddresses:\n{addr_output.stdout}\n{addr_output.stderr}"
                f"\nClient redirects:\n{client_redirects.stdout}\n{client_redirects.stderr}"
                f"\nClient nat-redirect:\n{nat_list.stdout}\n{nat_list.stderr}"
                f"\nNft fw4 prerouting:\n{fw4_pre.stdout}\n{fw4_pre.stderr}"
                f"\nNft fw4 output:\n{fw4_out.stdout}\n{fw4_out.stderr}"
                f"\nNft:\n{nat_table.stdout}\n{nat_table.stderr}"
            )
        if blackhole_added:
            client_host.run(f"ip route del blackhole {C1_LAN_CIDR} >/dev/null 2>&1 || true")
            blackhole_added = False

        client_runner(
            "nat-redirect",
            "add",
            "--cidr",
            C1_LAN_CIDR,
            "--quiet",
            check=True,
        )
        nat_added = True
        with _timed("nat-redirect warmup"):
            time.sleep(2.0)

        with _timed("ping with nat-redirect (gateway)"):
            nat_ping = client_runner(
                "ping",
                C1_LAN_GATEWAY,
                "--count",
                "1",
                check=True,
            )
        tunnel_common.assert_zero_loss(nat_ping, "nat-redirect to gateway")

        with _timed("ping with nat-redirect (c1 ip)"):
            nat_c1 = client_runner(
                "ping",
                c1_ip,
                "--count",
                "1",
                check=True,
            )
        tunnel_common.assert_zero_loss(nat_c1, f"nat-redirect to {c1_ip}")

        with _timed("c2 -> c1 ping"):
            c2_ping = openwrt_env.run_alpine_guest_script(
                alpine_c2_host,
                "scripts/linux/xp2p_ping.sh",
                c1_ip,
                "--count",
                "1",
            )
        if c2_ping.rc != 0:
            c2_net_dump = openwrt_env.run_alpine_guest_script(
                alpine_c2_host, "scripts/linux/net_dump.sh"
            )
            client_nat = client_runner("nat-redirect", "list", check=False).stdout or ""
            client_chain = chain_environment["client_host"].run("nft list table inet fw4 || true")
            pytest.fail(
                "c2 -> c1 ping failed.\n"
                f"STDOUT:\n{c2_ping.stdout}\nSTDERR:\n{c2_ping.stderr}"
                f"\nClient nat-redirect:\n{client_nat}"
                f"\nClient nft:\n{client_chain.stdout}\n{client_chain.stderr}"
                f"\nC2 net dump:\n{c2_net_dump.stdout}\n{c2_net_dump.stderr}"
            )
    finally:
        if route_backup:
            for route in route_backup:
                client_host.run(f"ip route replace {route} >/dev/null 2>&1 || true")
        if nat_added:
            client_runner("nat-redirect", "remove", "--all", check=False)
        if blackhole_added:
            client_host.run(f"ip route del blackhole {C1_LAN_CIDR} >/dev/null 2>&1 || true")
        if previous_server_mode and previous_server_mode != "proxy":
            _set_mode(server_runner, "server", helpers.SERVER_CONFIG_DIR_NAME, previous_server_mode)
            _apply_pending_config(server_host, "server")
        if previous_client_mode and previous_client_mode != "proxy":
            _set_mode(client_runner, "client", helpers.CLIENT_CONFIG_DIR_NAME, previous_client_mode)
            _apply_pending_config(client_host, "client")
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
                C1_LAN_CIDR,
                "--tag",
                endpoint_tag,
                check=False,
            )
        if diag_started:
            _alpine_guest(alpine_c1_host, "scripts/linux/stop_xp2p_diag.sh")
        elapsed = time.perf_counter() - test_start
        print(f"TIMING: test_chain_c2_b_a_c1_redirect_nat total: {elapsed:.2f}s")


def test_chain_c1_a_b_c2_reverse(chain_environment, alpine_c1_host, alpine_c2_host):
    test_start = time.perf_counter()
    server_runner = chain_environment["server_runner"]
    client_runner = chain_environment["client_runner"]
    reverse_tag = chain_environment["reverse_tag"]
    endpoint_tag = chain_environment["endpoint_tag"]
    client_user = chain_environment["client_user"]
    client_primary_ip = chain_environment["client_primary_ip"]
    server_redirect_cidr = None
    redirect_added = False
    nat_added = False
    diag_started = False
    server_host = chain_environment["server_host"]
    client_host = chain_environment["client_host"]
    previous_server_mode = None
    previous_client_mode = None

    with _timed("reverse guest route setup"):
        _alpine_guest(alpine_c1_host, "scripts/linux/ensure_route.sh", "10.0.102.0/24", C1_LAN_GATEWAY)
        _alpine_guest(alpine_c2_host, "scripts/linux/ensure_route.sh", "10.0.101.0/24", "10.0.102.1")
    with _timed("reverse guest diag start"):
        c2_ip = _alpine_guest(alpine_c2_host, "scripts/linux/get_interface_ip.sh", "eth1").stdout.strip()
    server_redirect_cidr = f"{c2_ip}/32"
    _alpine_guest(alpine_c2_host, "scripts/linux/start_xp2p_diag.sh", DIAG_LISTEN, "tcp")
    diag_started = True

    try:
        with _timed("reverse ensure proxy mode"):
            previous_server_mode = _ensure_mode(
                server_host, server_runner, "server", helpers.SERVER_CONFIG_DIR_NAME, "proxy"
            )
            previous_client_mode = _ensure_mode(
                client_host, client_runner, "client", helpers.CLIENT_CONFIG_DIR_NAME, "proxy"
            )
            _apply_pending_config(server_host, "server")
            _apply_pending_config(client_host, "client")
            helpers.wait_for_live_config(server_host, "server")
            helpers.wait_for_live_config(client_host, "client")
        with _timed("reverse baseline ping"):
            base_ping = openwrt_env.run_alpine_guest_script(
                alpine_c1_host,
                "scripts/linux/xp2p_ping.sh",
                c2_ip,
                "--count",
                "1",
            )
        if base_ping.rc == 0:
            pytest.fail(
                "expected ping to fail without reverse redirect.\n"
                f"STDOUT:\n{base_ping.stdout}\nSTDERR:\n{base_ping.stderr}"
            )

        with _timed("reverse add server redirect"):
            server_runner(
                "server",
                "redirect",
                "add",
                "--path",
                helpers.INSTALL_ROOT.as_posix(),
                "--config-dir",
                helpers.SERVER_CONFIG_DIR_NAME,
                "--cidr",
                server_redirect_cidr,
                "--tag",
                reverse_tag,
                check=True,
            )
        redirect_added = True

        with _timed("reverse add nat-redirect"):
            server_runner(
                "nat-redirect",
                "add",
                "--cidr",
                server_redirect_cidr,
                "--quiet",
                check=True,
            )
        nat_added = True

        helpers.ensure_service_running(chain_environment["server_host"], "server")
        helpers.ensure_service_running(chain_environment["client_host"], "client")
        helpers.wait_for_apply_request_clear(chain_environment["server_host"], timeout_seconds=60.0)
        helpers.wait_for_apply_request_clear(chain_environment["client_host"], timeout_seconds=60.0)
        helpers.wait_for_live_config(chain_environment["server_host"], "server")
        helpers.wait_for_live_config(chain_environment["client_host"], "client")
        with _timed("reverse tunnel warmup"):
            time.sleep(2.0)
        server_state = helpers.read_live_server_config(server_host)
        server_routing = helpers.read_live_json(server_host, helpers.SERVER_CONFIG_DIR / "routing.json")
        helpers.assert_server_redirect_state(server_state, server_redirect_cidr, reverse_tag)
        helpers.assert_server_redirect_rule(server_routing, server_redirect_cidr, reverse_tag)
        with _timed("reverse wait heartbeat"):
            tunnel_common.wait_for_alive_entry(
                server_runner,
                "server",
                chain_environment["server_install_path"],
                endpoint_tag,
                SERVER_TUNNEL_IP,
                client_user,
                client_primary_ip,
            )

        with _timed("reverse ping c1 -> c2"):
            c1_ping = openwrt_env.run_alpine_guest_script(
                alpine_c1_host,
                "scripts/linux/xp2p_ping.sh",
                c2_ip,
                "--count",
                "1",
            )
        if c1_ping.rc != 0:
            c1_net_dump = openwrt_env.run_alpine_guest_script(
                alpine_c1_host, "scripts/linux/net_dump.sh"
            )
            server_state_output = server_runner(
                "server", "state", "--path", helpers.INSTALL_ROOT.as_posix(), check=True
            ).stdout or ""
            client_state_output = chain_environment["client_runner"](
                "client", "state", "--path", helpers.INSTALL_ROOT.as_posix(), check=True
            ).stdout or ""
            server_nat = server_runner("nat-redirect", "list", check=False).stdout or ""
            server_chain = server_host.run("nft list table inet fw4 || true")
            client_chain = chain_environment["client_host"].run("nft list table inet fw4 || true")
            pytest.fail(
                "c1 -> c2 reverse ping failed.\n"
                f"STDOUT:\n{c1_ping.stdout}\nSTDERR:\n{c1_ping.stderr}"
                f"\nC1 net dump:\n{c1_net_dump.stdout}\n{c1_net_dump.stderr}"
                f"\nServer state:\n{server_state_output}"
                f"\nClient state:\n{client_state_output}"
                f"\nServer nat-redirect:\n{server_nat}"
                f"\nServer nft:\n{server_chain.stdout}\n{server_chain.stderr}"
                f"\nClient nft:\n{client_chain.stdout}\n{client_chain.stderr}"
            )
    finally:
        if previous_server_mode and previous_server_mode != "proxy":
            _set_mode(server_runner, "server", helpers.SERVER_CONFIG_DIR_NAME, previous_server_mode)
            _apply_pending_config(server_host, "server")
        if previous_client_mode and previous_client_mode != "proxy":
            _set_mode(client_runner, "client", helpers.CLIENT_CONFIG_DIR_NAME, previous_client_mode)
            _apply_pending_config(client_host, "client")
        if nat_added:
            server_runner("nat-redirect", "remove", "--all", check=False)
        if redirect_added:
            server_runner(
                "server",
                "redirect",
                "remove",
                "--path",
                helpers.INSTALL_ROOT.as_posix(),
                "--config-dir",
                helpers.SERVER_CONFIG_DIR_NAME,
                "--cidr",
                server_redirect_cidr,
                "--tag",
                reverse_tag,
                check=False,
            )
        if diag_started:
            _alpine_guest(alpine_c2_host, "scripts/linux/stop_xp2p_diag.sh")
        elapsed = time.perf_counter() - test_start
        print(f"TIMING: test_chain_c1_a_b_c2_reverse total: {elapsed:.2f}s")
