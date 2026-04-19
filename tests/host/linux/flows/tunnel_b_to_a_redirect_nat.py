from __future__ import annotations

import time

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux.flows import tunnel_b_to_a_fixture as fixture
from tests.host.linux.flows import tunnel_b_to_a_nat_helpers as nat_helpers
from tests.host.tunnel import common as tunnel_common


def assert_client_and_server_redirect_with_nat(env: dict) -> None:
    client_runner = env["client_runner"]
    server_runner = env["server_runner"]
    client_host = env["client_host"]
    server_host = env["server_host"]
    server_install_path = env["server_install_path"]
    endpoint_tag = env["endpoint_tag"]
    reverse_tag = env["reverse_tag"]
    nat_snippet = "/etc/nftables.d/xray-transparent.nft"
    nat_entries = "/etc/nftables.d/xray-transparent.d"
    client_target_alias = "10.0.101.50/32"
    server_target_alias = "10.0.102.50/32"
    chain_name = "xray_transparent_prerouting"
    client_listener_port = fixture.SERVER_DIAGNOSTICS_PORT
    server_listener_port = fixture.CLIENT_DIAGNOSTICS_PORT
    client_nat_runner = fixture.nat_runner(client_host, role="client")
    server_nat_runner = fixture.nat_runner(server_host, role="server")
    debug_ctx = nat_helpers.NatDebugContext(
        client_host=client_host,
        server_host=server_host,
        client_nat_runner=client_nat_runner,
        server_nat_runner=server_nat_runner,
        nat_snippet=nat_snippet,
        nat_entries=nat_entries,
    )
    redirect_added = False
    server_redirect_added = False
    client_proxy_mode = False
    server_proxy_mode = False

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

    try:
        client_runner(
            "client",
            "mode",
            "proxy",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            check=True,
        )
        client_proxy_mode = True
        server_runner(
            "server",
            "mode",
            "proxy",
            "--path",
            server_install_path,
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            check=True,
        )
        server_proxy_mode = True

        client_runner(
            "client",
            "redirect",
            "add",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--cidr",
            fixture.CLIENT_REDIRECT_CIDR,
            "--tag",
            endpoint_tag,
            "--quiet",
            check=True,
        )
        redirect_added = True

        client_state = helpers.read_pending_client_config(client_host)
        client_routing = helpers.render_xray(client_host, client_runner, "client", desired=True)
        helpers.assert_redirect_rule(client_routing, fixture.CLIENT_REDIRECT_CIDR, endpoint_tag)
        helpers.assert_client_reverse_state(
            client_state,
            env["reverse_tag"],
            endpoint_tag=endpoint_tag,
            user=env["client_user"],
            host=fixture.SERVER_IP,
        )
        dokodemo_ports = _dokodemo_ports({"inbounds": client_routing.get("inbounds") or []})
        assert dokodemo_ports, "Expected dokodemo-door followRedirect inbound for transparent mode"

        with fixture.active_tunnel_sessions(env):
            client_inbounds = helpers.read_json(client_host, helpers.CLIENT_LIVE_DIR / "xray.json")
            client_dokodemo_ports = _dokodemo_ports(client_inbounds)
            assert client_dokodemo_ports, (
                "Expected dokodemo-door with followRedirect in client xray.json inbounds: "
                f"{client_inbounds}"
            )
            server_inbounds = helpers.read_json(server_host, helpers.SERVER_LIVE_DIR / "xray.json")
            server_dokodemo_ports = _dokodemo_ports(server_inbounds)
            assert server_dokodemo_ports, (
                "Expected dokodemo-door with followRedirect in server xray.json inbounds: "
                f"{server_inbounds}"
            )

        plan_output = client_nat_runner(
            "nat-redirect",
            "add",
            "--cidr",
            fixture.CLIENT_REDIRECT_CIDR,
            "--inbounds",
            (helpers.CLIENT_LIVE_DIR / "xray.json").as_posix(),
            "--print-only",
            "--quiet",
            check=True,
        ).stdout or ""
        assert nat_snippet in plan_output
        assert nat_entries in plan_output
        client_nat_runner(
            "nat-redirect",
            "add",
            "--cidr",
            fixture.CLIENT_REDIRECT_CIDR,
            "--inbounds",
            (helpers.CLIENT_LIVE_DIR / "xray.json").as_posix(),
            "--quiet",
            check=True,
        )
        time.sleep(2.0)
        nat_list = client_nat_runner("nat-redirect", "list", check=True).stdout or ""
        assert fixture.CLIENT_REDIRECT_CIDR in nat_list

        with fixture.ip_alias(server_host, client_target_alias):
            target_ip = client_target_alias.split("/")[0]
            with fixture.active_tunnel_sessions(env):
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
                    f"through SOCKS tunnel before redirect ping. {nat_helpers.nat_debug(debug_ctx)}",
                )
                if not nat_helpers.has_nat_chain(client_host, chain_name=chain_name):
                    pytest.fail(
                        "transparent NAT backend not available on client host (no nft/iptables chain).\n"
                        f"{nat_helpers.nat_debug(debug_ctx)}"
                    )
                initial_packets = nat_helpers.traffic_counter_sum(client_host, chain_name=chain_name)
                ping_result = None
                for _ in range(3):
                    ping_result = client_runner(
                        "ping",
                        target_ip,
                        "--port",
                        str(client_listener_port),
                        "--count",
                        "3",
                        "--proto",
                        "tcp",
                        check=False,
                    )
                    if ping_result.rc == 0:
                        break
                    time.sleep(2.0)
                if ping_result is None or ping_result.rc != 0:
                    pytest.fail(
                        "xp2p ping via client redirect failed.\n"
                        f"STDOUT:\n{ping_result.stdout if ping_result else ''}\n"
                        f"STDERR:\n{ping_result.stderr if ping_result else ''}\n"
                        f"{nat_helpers.nat_debug(debug_ctx)}"
                    )
                tunnel_common.assert_zero_loss(
                    ping_result,
                    f"while redirecting through server. {nat_helpers.nat_debug(debug_ctx)}",
                )
                final_packets = nat_helpers.traffic_counter_sum(client_host, chain_name=chain_name)
                assert final_packets > initial_packets, (
                    "transparent redirect counters did not increase after redirect ping. "
                    f"initial={initial_packets} final={final_packets}. {nat_helpers.nat_debug(debug_ctx)}"
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
            fixture.SERVER_REDIRECT_CIDR,
            "--tag",
            reverse_tag,
            check=True,
        )
        server_redirect_added = True
        server_nat_runner(
            "nat-redirect",
            "add",
            "--cidr",
            fixture.SERVER_REDIRECT_CIDR,
            "--inbounds",
            (helpers.SERVER_LIVE_DIR / "xray.json").as_posix(),
            "--quiet",
            check=True,
        )
        time.sleep(2.0)
        server_nat_list = server_nat_runner("nat-redirect", "list", check=True).stdout or ""
        assert fixture.SERVER_REDIRECT_CIDR in server_nat_list

        with fixture.ip_alias(client_host, server_target_alias):
            reverse_ip = server_target_alias.split("/")[0]
            with fixture.active_tunnel_sessions(env):
                server_socks_addr = f"127.0.0.1:{fixture.socks_port(server_host, helpers.SERVER_LIVE_DIR / 'xray.json')}"
                socks_ping = server_runner(
                    "ping",
                    reverse_ip,
                    f"--tunnel={server_socks_addr}",
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
                    f"server-side SOCKS ping toward {reverse_ip}. {nat_helpers.nat_debug(debug_ctx)}",
                )
                if not nat_helpers.has_nat_chain(server_host, chain_name=chain_name):
                    pytest.fail(
                        "transparent NAT backend not available on server host (no nft/iptables chain).\n"
                        f"{nat_helpers.nat_debug(debug_ctx)}"
                    )
                initial_server_packets = nat_helpers.traffic_counter_sum(server_host, chain_name=chain_name)
                ping_result = None
                for _ in range(3):
                    ping_result = server_runner(
                        "ping",
                        reverse_ip,
                        "--port",
                        str(server_listener_port),
                        "--count",
                        "3",
                        "--proto",
                        "tcp",
                        check=False,
                    )
                    if ping_result.rc == 0:
                        break
                    time.sleep(2.0)
                if ping_result is None or ping_result.rc != 0:
                    pytest.fail(
                        "xp2p ping via server redirect failed.\n"
                        f"STDOUT:\n{ping_result.stdout if ping_result else ''}\n"
                        f"STDERR:\n{ping_result.stderr if ping_result else ''}\n"
                        f"{nat_helpers.nat_debug(debug_ctx)}"
                    )
                tunnel_common.assert_zero_loss(
                    ping_result,
                    f"server-side direct ping toward {reverse_ip}. {nat_helpers.nat_debug(debug_ctx)}",
                )
                final_server_packets = nat_helpers.traffic_counter_sum(server_host, chain_name=chain_name)
                assert final_server_packets > initial_server_packets, (
                    "server transparent redirect counters did not increase after reverse ping. "
                    f"initial={initial_server_packets} final={final_server_packets}. {nat_helpers.nat_debug(debug_ctx)}"
                )
    finally:
        client_nat_runner("nat-redirect", "remove", "--all", check=False)
        server_nat_runner("nat-redirect", "remove", "--all", check=False)
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
                fixture.CLIENT_REDIRECT_CIDR,
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
                fixture.SERVER_REDIRECT_CIDR,
                "--tag",
                reverse_tag,
                "--quiet",
                check=False,
            )
        if client_proxy_mode:
            client_runner(
                "client",
                "mode",
                "tun",
                "--path",
                helpers.INSTALL_ROOT.as_posix(),
                "--config-dir",
                helpers.CLIENT_CONFIG_DIR_NAME,
                check=False,
            )
        if server_proxy_mode:
            server_runner(
                "server",
                "mode",
                "tun",
                "--path",
                server_install_path,
                "--config-dir",
                helpers.SERVER_CONFIG_DIR_NAME,
                check=False,
            )
        fixture.assert_redirect_entries_removed(
            env,
            client_cidr=fixture.CLIENT_REDIRECT_CIDR,
            client_tag=endpoint_tag,
            server_cidr=fixture.SERVER_REDIRECT_CIDR,
            server_tag=reverse_tag,
        )
