from __future__ import annotations

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


def _alpine_guest(host, script: str, *args: str):
    result = openwrt_env.run_alpine_guest_script(host, script, *args)
    if result.rc != 0:
        pytest.fail(
            "guest script failed "
            f"(exit {result.rc}).\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return result


def _current_mode(host, role: str) -> str:
    if role == "server":
        config = helpers.read_server_config(host)
    elif role == "client":
        config = helpers.read_client_config(host)
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

    _alpine_guest(alpine_c1_host, "scripts/linux/ensure_route.sh", "10.0.102.0/24", C1_LAN_GATEWAY)
    _alpine_guest(alpine_c2_host, "scripts/linux/ensure_route.sh", "10.0.101.0/24", "10.0.102.1")
    c1_ip = _alpine_guest(alpine_c1_host, "scripts/linux/get_interface_ip.sh", "eth1").stdout.strip()
    _alpine_guest(alpine_c1_host, "scripts/linux/start_xp2p_diag.sh", DIAG_LISTEN, "tcp")
    diag_started = True

    try:
        previous_server_mode = _ensure_mode(
            server_host, server_runner, "server", helpers.SERVER_CONFIG_DIR_NAME, "proxy"
        )
        previous_client_mode = _ensure_mode(
            client_host, client_runner, "client", helpers.CLIENT_CONFIG_DIR_NAME, "proxy"
        )
        initial_ping = server_runner(
            "ping",
            c1_ip,
            "--count",
            "1",
            check=True,
        )
        tunnel_common.assert_zero_loss(initial_ping, f"direct ping to {c1_ip}")

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

        with openwrt_env.xp2p_run_session(
            chain_environment["server_host"],
            "server",
            chain_environment["server_install_path"],
            helpers.SERVER_CONFIG_DIR_NAME,
            helpers.SERVER_LOG_FILE,
        ), openwrt_env.xp2p_run_session(
            chain_environment["client_host"],
            "client",
            helpers.INSTALL_ROOT.as_posix(),
            helpers.CLIENT_CONFIG_DIR_NAME,
            helpers.CLIENT_LOG_FILE,
        ):
            time.sleep(2.0)
            tunnel_ping = client_runner(
                "ping",
                SERVER_TUNNEL_IP,
                "--tunnel",
                "--count",
                "1",
                check=True,
            )
            tunnel_common.assert_zero_loss(tunnel_ping, "through SOCKS tunnel")

            redirect_ping = client_runner(
                "ping",
                C1_LAN_GATEWAY,
                "--tunnel",
                "--count",
                "1",
                check=True,
            )
            tunnel_common.assert_zero_loss(redirect_ping, "redirect via SOCKS tunnel")

            missing_nat = client_runner(
                "ping",
                C1_LAN_GATEWAY,
                "--count",
                "1",
                check=False,
            )
            if missing_nat.rc == 0:
                pytest.fail(
                    "expected ping to fail without nat-redirect.\n"
                    f"STDOUT:\n{missing_nat.stdout}\nSTDERR:\n{missing_nat.stderr}"
                )

            client_runner(
                "nat-redirect",
                "add",
                "--cidr",
                C1_LAN_CIDR,
                "--quiet",
                check=True,
            )
            nat_added = True
            time.sleep(2.0)

            nat_ping = client_runner(
                "ping",
                C1_LAN_GATEWAY,
                "--count",
                "1",
                check=True,
            )
            tunnel_common.assert_zero_loss(nat_ping, "nat-redirect to gateway")

            nat_c1 = client_runner(
                "ping",
                c1_ip,
                "--count",
                "1",
                check=True,
            )
            tunnel_common.assert_zero_loss(nat_c1, f"nat-redirect to {c1_ip}")

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
        if previous_server_mode and previous_server_mode != "proxy":
            _set_mode(server_runner, "server", helpers.SERVER_CONFIG_DIR_NAME, previous_server_mode)
        if previous_client_mode and previous_client_mode != "proxy":
            _set_mode(client_runner, "client", helpers.CLIENT_CONFIG_DIR_NAME, previous_client_mode)
        if nat_added:
            client_runner("nat-redirect", "remove", "--all", check=False)
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


def test_chain_c1_a_b_c2_reverse(chain_environment, alpine_c1_host, alpine_c2_host):
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

    _alpine_guest(alpine_c1_host, "scripts/linux/ensure_route.sh", "10.0.102.0/24", C1_LAN_GATEWAY)
    _alpine_guest(alpine_c2_host, "scripts/linux/ensure_route.sh", "10.0.101.0/24", "10.0.102.1")
    c2_ip = _alpine_guest(alpine_c2_host, "scripts/linux/get_interface_ip.sh", "eth1").stdout.strip()
    server_redirect_cidr = f"{c2_ip}/32"
    _alpine_guest(alpine_c2_host, "scripts/linux/start_xp2p_diag.sh", DIAG_LISTEN, "tcp")
    diag_started = True

    try:
        previous_server_mode = _ensure_mode(
            server_host, server_runner, "server", helpers.SERVER_CONFIG_DIR_NAME, "proxy"
        )
        previous_client_mode = _ensure_mode(
            client_host, client_runner, "client", helpers.CLIENT_CONFIG_DIR_NAME, "proxy"
        )
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
        server_state = helpers.read_server_config(server_host)
        server_routing = helpers.read_json(server_host, helpers.SERVER_CONFIG_DIR / "routing.json")
        helpers.assert_server_redirect_state(server_state, server_redirect_cidr, reverse_tag)
        helpers.assert_server_redirect_rule(server_routing, server_redirect_cidr, reverse_tag)

        server_runner(
            "nat-redirect",
            "add",
            "--cidr",
            server_redirect_cidr,
            "--quiet",
            check=True,
        )
        nat_added = True

        with openwrt_env.xp2p_run_session(
            chain_environment["server_host"],
            "server",
            chain_environment["server_install_path"],
            helpers.SERVER_CONFIG_DIR_NAME,
            helpers.SERVER_LOG_FILE,
        ), openwrt_env.xp2p_run_session(
            chain_environment["client_host"],
            "client",
            helpers.INSTALL_ROOT.as_posix(),
            helpers.CLIENT_CONFIG_DIR_NAME,
            helpers.CLIENT_LOG_FILE,
        ):
            time.sleep(2.0)
            tunnel_common.wait_for_alive_entry(
                server_runner,
                "server",
                chain_environment["server_install_path"],
                endpoint_tag,
                SERVER_TUNNEL_IP,
                client_user,
                client_primary_ip,
            )

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
        if previous_client_mode and previous_client_mode != "proxy":
            _set_mode(client_runner, "client", helpers.CLIENT_CONFIG_DIR_NAME, previous_client_mode)
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
