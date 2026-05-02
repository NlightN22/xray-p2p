from __future__ import annotations

import time

import pytest

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env
from tests.host.openwrt.flows import tunnel_b_to_a_actions as actions
from tests.host.openwrt.flows import tunnel_b_to_a_fixture as fixture
from tests.host.openwrt.flows import tunnel_b_to_a_waits as waits
from tests.host.tunnel import common as tunnel_common


def run_forward_tunnel_operational(env: dict) -> None:
    client_runner = env["client_runner"]

    with fixture.active_tunnel_sessions(env):
        ping_result = client_runner(
            "ping",
            waits.SERVER_IP,
            "--tunnel",
            "--count",
            "3",
            check=True,
        )
        tunnel_common.assert_zero_loss(ping_result, "through SOCKS tunnel")
        actions.verify_heartbeat_state(env)
        actions.run_server_state_watch(env)
    actions.exercise_client_forward_diagnostics(env, fixture.active_tunnel_sessions)
    actions.exercise_server_forward_diagnostics(env, fixture.active_tunnel_sessions)


def run_client_redirect_through_server(env: dict) -> None:
    client_runner = env["client_runner"]
    client_host = env["client_host"]
    server_host = env["server_host"]
    server_runner = env["server_runner"]
    server_install_path = env["server_install_path"]
    endpoint_tag = env["endpoint_tag"]
    nat_snippet = "/etc/nftables.d/xray-transparent.nft"
    nat_entries = "/etc/nftables.d/xray-transparent.d"
    target_alias = "10.0.101.50/32"
    listener_port = waits.SERVER_DIAGNOSTICS_PORT
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
        waits.CLIENT_REDIRECT_CIDR,
        "--tag",
        endpoint_tag,
        check=True,
    )
    actions.dump_apply_state(client_host, "client", "after redirect add (before apply)")
    waits.apply_pending_config_wait(
        client_host,
        "client",
        helpers.INSTALL_ROOT.as_posix(),
        helpers.CLIENT_CONFIG_DIR_NAME,
    )
    waits.wait_for_live_config(client_host, "client")
    actions.dump_apply_state(client_host, "client", "after redirect apply")
    redirect_added = True
    server_redirect_added = False
    server_nat_added = False
    nat_added = False
    server_redirect_cidr: str | None = None
    try:
        previous_server_mode = waits.ensure_mode(
            server_host, server_runner, "server", helpers.SERVER_CONFIG_DIR_NAME, "proxy"
        )
        previous_client_mode = waits.ensure_mode(
            client_host, client_runner, "client", helpers.CLIENT_CONFIG_DIR_NAME, "proxy"
        )
        waits.apply_pending_config_wait(
            server_host,
            "server",
            server_install_path,
            helpers.SERVER_CONFIG_DIR_NAME,
        )
        waits.wait_for_live_config(server_host, "server")
        waits.apply_pending_config_wait(
            client_host,
            "client",
            helpers.INSTALL_ROOT.as_posix(),
            helpers.CLIENT_CONFIG_DIR_NAME,
        )
        waits.wait_for_live_config(client_host, "client")
        with fixture.active_tunnel_sessions(env):
            client_state = helpers.read_live_client_config(client_host)
            client_routing = helpers.read_live_json(client_host, helpers.CLIENT_CONFIG_DIR / "routing.json")
            helpers.assert_redirect_rule(client_routing, waits.CLIENT_REDIRECT_CIDR, endpoint_tag)
            helpers.assert_client_reverse_state(
                client_state,
                env["reverse_tag"],
                endpoint_tag=endpoint_tag,
                user=env["client_user"],
                host=waits.SERVER_IP,
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

        client_dokodemo_port = int(client_dokodemo_ports[0])
        server_dokodemo_port = int(server_dokodemo_ports[0])
        plan_output = client_runner(
            "nat-redirect",
            "add",
            "--cidr",
            waits.CLIENT_REDIRECT_CIDR,
            "--port",
            str(client_dokodemo_port),
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
            waits.CLIENT_REDIRECT_CIDR,
            "--port",
            str(client_dokodemo_port),
            "--quiet",
            check=True,
        )
        nat_added = True
        actions.dump_apply_state(client_host, "client", "after nat-redirect add (before apply)")
        waits.apply_pending_config_wait(
            client_host,
            "client",
            helpers.INSTALL_ROOT.as_posix(),
            helpers.CLIENT_CONFIG_DIR_NAME,
        )
        waits.wait_for_live_config(client_host, "client")
        actions.dump_apply_state(client_host, "client", "after nat-redirect apply")
        time.sleep(2.0)
        nat_list = client_runner("nat-redirect", "list", check=True).stdout or ""
        assert waits.CLIENT_REDIRECT_CIDR in nat_list

        with actions.ip_alias(server_host, target_alias):
            with fixture.active_tunnel_sessions(env):
                target_ip = target_alias.split("/")[0]
                waits.wait_for_listen_port(server_host, listener_port)
                actions.assert_server_alias_reachable(server_host, server_runner, target_ip, listener_port)
                server_nat_dump = server_runner("nat-redirect", "list", check=True).stdout or ""
                client_nat_dump = client_runner("nat-redirect", "list", check=True).stdout or ""
                server_chain = server_host.run("nft list chain inet fw4 xray_transparent_prerouting")
                client_chain = client_host.run("nft list chain inet fw4 xray_transparent_prerouting")
                client_socks_netstat = client_host.run("netstat -lpn 2>/dev/null | grep ':51180' || true")
                client_processes = client_host.run("ps w | grep -E 'xp2p|xray' | grep -v grep")
                actions.dump_apply_state(client_host, "client", "before socks ping")
                actions.dump_apply_state(server_host, "server", "before socks ping")
                waits.wait_for_socks_ready(client_host, timeout_seconds=10.0)
                waits.wait_for_ping_ready(
                    client_runner,
                    target_ip,
                    proto="tcp",
                    tunnel=True,
                    timeout_seconds=10.0,
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
                actions.verify_heartbeat_state(env)
            with fixture.active_tunnel_sessions(env):
                waits.wait_for_ping_ready(client_runner, target_ip, proto="tcp", timeout_seconds=30.0)
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

        server_redirect_cidr = waits.SERVER_REDIRECT_CIDR
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
            env["reverse_tag"],
            check=True,
        )
        server_redirect_added = True
        actions.dump_apply_state(server_host, "server", "after server redirect add (before apply)")
        waits.apply_pending_config_wait(
            server_host,
            "server",
            server_install_path,
            helpers.SERVER_CONFIG_DIR_NAME,
        )
        actions.dump_apply_state(server_host, "server", "after server redirect apply")
        server_runner(
            "nat-redirect",
            "add",
            "--cidr",
            server_redirect_cidr,
            "--port",
            str(server_dokodemo_port),
            "--quiet",
            check=True,
        )
        server_nat_added = True
        actions.dump_apply_state(server_host, "server", "after server nat-redirect add (before apply)")
        waits.apply_pending_config_wait(
            server_host,
            "server",
            server_install_path,
            helpers.SERVER_CONFIG_DIR_NAME,
        )
        actions.dump_apply_state(server_host, "server", "after server nat-redirect apply")
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
                waits.CLIENT_REDIRECT_CIDR,
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
                server_redirect_cidr or waits.CLIENT_REDIRECT_CIDR,
                "--tag",
                env["reverse_tag"],
                check=False,
            )
        waits.apply_pending_config_wait(
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
        assert waits.CLIENT_REDIRECT_CIDR not in final_list
        if previous_server_mode and previous_server_mode != "proxy":
            waits.set_mode(server_runner, "server", helpers.SERVER_CONFIG_DIR_NAME, previous_server_mode)
            waits.apply_pending_config_wait(
                server_host,
                "server",
                server_install_path,
                helpers.SERVER_CONFIG_DIR_NAME,
            )
        if previous_client_mode and previous_client_mode != "proxy":
            waits.set_mode(client_runner, "client", helpers.CLIENT_CONFIG_DIR_NAME, previous_client_mode)
            waits.apply_pending_config_wait(
                client_host,
                "client",
                helpers.INSTALL_ROOT.as_posix(),
                helpers.CLIENT_CONFIG_DIR_NAME,
            )


def run_reverse_redirect_via_server_portal(env: dict) -> None:
    server_runner = env["server_runner"]
    server_install_path = env["server_install_path"]
    reverse_tag = env["reverse_tag"]
    client_host = env["client_host"]
    server_host = env["server_host"]
    previous_server_mode = None

    alias_cidr = f"{waits.CLIENT_REVERSE_TEST_IP}/32"
    with actions.ip_alias(client_host, alias_cidr):
        previous_server_mode = waits.ensure_mode(
            server_host, server_runner, "server", helpers.SERVER_CONFIG_DIR_NAME, "proxy"
        )
        waits.apply_pending_config_wait(
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
        actions.dump_apply_state(server_host, "server", "after reverse redirect add (before apply)")
        waits.apply_pending_config_wait(
            server_host,
            "server",
            server_install_path,
            helpers.SERVER_CONFIG_DIR_NAME,
        )
        actions.dump_apply_state(server_host, "server", "after reverse redirect apply")
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
            with fixture.active_tunnel_sessions(env):
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
                f"{waits.CLIENT_REVERSE_TEST_IP}:{waits.CLIENT_DIAGNOSTICS_PORT}",
                "--listen",
                "127.0.0.1",
                "--listen-port",
                str(waits.SERVER_FORWARD_PORT),
                "--proto",
                "tcp",
                check=True,
            )
            forward_added = True
            actions.dump_apply_state(server_host, "server", "after forward add (before apply)")
            waits.apply_pending_config_wait(
                server_host,
                "server",
                server_install_path,
                helpers.SERVER_CONFIG_DIR_NAME,
            )
            actions.dump_apply_state(server_host, "server", "after forward apply")

            server_state = helpers.read_live_server_config(server_host)
            entry = tunnel_common.forward_entry_for_target(
                server_state.get("forward_rules") or [], waits.CLIENT_REVERSE_TEST_IP, waits.CLIENT_DIAGNOSTICS_PORT
            )
            listen_port = tunnel_common.listen_port_from_entry(entry)
            assert listen_port == waits.SERVER_FORWARD_PORT

            ping_result = None
            last_error: BaseException | None = None
            for attempt in range(2):
                with fixture.active_tunnel_sessions(env):
                    waits.wait_for_listen_port(server_host, waits.SERVER_FORWARD_PORT)
                    waits.wait_for_listen_port(client_host, waits.CLIENT_DIAGNOSTICS_PORT)
                    time.sleep(2.5)
                    try:
                        waits.wait_for_ping_ready(
                            server_runner,
                            "127.0.0.1",
                            port=waits.SERVER_FORWARD_PORT,
                            proto="tcp",
                            timeout_seconds=60.0,
                        )
                        ping_result = server_runner(
                            "ping",
                            "127.0.0.1",
                            "--port",
                            str(waits.SERVER_FORWARD_PORT),
                            "--count",
                            "3",
                            "--proto",
                            "tcp",
                            check=True,
                        )
                        tunnel_common.assert_zero_loss(
                            ping_result, f"via server forward targeting {waits.CLIENT_REVERSE_TEST_IP}"
                        )
                        actions.verify_heartbeat_state(env)
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
                    waits.ensure_service_running(server_host, "server")
                    waits.ensure_service_running(client_host, "client")
            if last_error is not None:
                raise last_error
        finally:
            if forward_added:
                actions.server_forward_cmd(
                    env,
                    "remove",
                    "--listen-port",
                    str(waits.SERVER_FORWARD_PORT),
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
            waits.apply_pending_config_wait(
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
                waits.set_mode(server_runner, "server", helpers.SERVER_CONFIG_DIR_NAME, previous_server_mode)
                waits.apply_pending_config_wait(
                    server_host,
                    "server",
                    server_install_path,
                    helpers.SERVER_CONFIG_DIR_NAME,
                )

