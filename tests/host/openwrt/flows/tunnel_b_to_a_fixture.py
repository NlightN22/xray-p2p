from __future__ import annotations

from contextlib import contextmanager
import time

import pytest

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env
from tests.host.tunnel import common as tunnel_common
from tests.host.openwrt.flows import tunnel_b_to_a_actions as actions
from tests.host.openwrt.flows import tunnel_b_to_a_waits as waits


def runner(host):
    def _run(*args: str, check: bool = False):
        result = openwrt_env.run_xp2p_live(host, *args)
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
    server_runner = runner(server_host)
    client_runner = runner(client_host)
    server_install_path = helpers.INSTALL_ROOT.as_posix()
    client_primary_ip = helpers.detect_primary_ipv4(client_host)

    def cleanup():
        for host in (server_host, client_host):
            openwrt_env._stop_xp2p_services(host)
            host.run("pkill -f 'xp2p server run' >/dev/null 2>&1 || true")
            host.run("pkill -f 'xp2p client run' >/dev/null 2>&1 || true")
            host.run("pkill -f '/etc/xp2p/bin/xray' >/dev/null 2>&1 || true")
            host.run("nft delete table inet xray_transparent >/dev/null 2>&1 || true")
            host.run(
                "rm -f /etc/nftables.d/xray-transparent.nft /etc/xp2p/nftables/xray-transparent.nft >/dev/null 2>&1 || true"
            )
            host.run(
                "rm -f /etc/nftables.d/xray-transparent.d/*.entry /etc/xp2p/nftables/xray-transparent.d/*.entry >/dev/null 2>&1 || true"
            )
        helpers.cleanup_server_install(server_host, server_runner)
        helpers.cleanup_client_install(client_host, client_runner)
        helpers.remove_path(server_host, waits.SERVER_HEARTBEAT_STATE_FILE)
        helpers.remove_path(client_host, waits.CLIENT_HEARTBEAT_STATE_FILE)

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
            waits.SERVER_IP,
            "--force",
            check=True,
        )
        credential = helpers.extract_trojan_credential(server_install.stdout or "")
        assert credential["link"], "Expected trojan link in server install output"
        reverse_tag = helpers.expected_reverse_tag(credential["user"], waits.SERVER_IP)
        waits.set_mode(server_runner, "server", helpers.SERVER_CONFIG_DIR_NAME, "proxy")
        waits.apply_pending_config_wait(
            server_host,
            "server",
            server_install_path,
            helpers.SERVER_CONFIG_DIR_NAME,
        )
        waits.wait_for_live_config(server_host, "server")
        server_state = helpers.read_live_server_config(server_host)
        server_routing = helpers.read_live_json(server_host, helpers.SERVER_CONFIG_DIR / "routing.json")
        helpers.assert_server_reverse_state(
            server_state,
            reverse_tag,
            user=credential["user"],
            host=waits.SERVER_IP,
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
        waits.set_mode(client_runner, "client", helpers.CLIENT_CONFIG_DIR_NAME, "proxy")
        waits.apply_pending_config_wait(
            client_host,
            "client",
            helpers.INSTALL_ROOT.as_posix(),
            helpers.CLIENT_CONFIG_DIR_NAME,
        )
        waits.wait_for_live_config(client_host, "client")
        client_state = helpers.read_live_client_config(client_host)
        client_routing = helpers.read_live_json(client_host, helpers.CLIENT_CONFIG_DIR / "routing.json")
        endpoint_tag = helpers.expected_proxy_tag(waits.SERVER_IP)
        helpers.assert_client_reverse_artifacts(client_routing, reverse_tag, endpoint_tag)
        helpers.assert_client_reverse_state(
            client_state,
            reverse_tag,
            endpoint_tag=endpoint_tag,
            user=credential["user"],
            host=waits.SERVER_IP,
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
def active_tunnel_sessions(env: dict):
    heartbeat_timeout = 30.0
    heartbeat_interval = 2.0
    waits.ensure_service_running(env["server_host"], "server")
    waits.ensure_service_running(env["client_host"], "client")
    helpers.wait_for_apply_request_clear(env["server_host"], timeout_seconds=60.0)
    helpers.wait_for_apply_request_clear(env["client_host"], timeout_seconds=60.0)
    helpers.wait_for_live_config(env["server_host"], "server")
    helpers.wait_for_live_config(env["client_host"], "client")
    waits.wait_for_xray_configs(env["client_host"], helpers.CLIENT_CONFIG_DIR)
    waits.wait_for_xray_configs(env["server_host"], helpers.SERVER_CONFIG_DIR)
    actions.dump_run_logs(env["server_host"], "server", "before")
    actions.dump_run_logs(env["client_host"], "client", "before")
    time.sleep(2.0)
    actions.dump_run_logs(env["server_host"], "server", "after-start")
    actions.dump_run_logs(env["client_host"], "client", "after-start")
    waits.wait_for_listen_port(env["client_host"], waits.CLIENT_SOCKS_PORT)
    time.sleep(2.0)
    helpers.wait_for_heartbeat_state(
        env["server_host"],
        waits.SERVER_HEARTBEAT_STATE_FILE,
        timeout_seconds=heartbeat_timeout,
        poll_interval=heartbeat_interval,
    )
    helpers.wait_for_heartbeat_state(
        env["client_host"],
        waits.CLIENT_HEARTBEAT_STATE_FILE,
        timeout_seconds=heartbeat_timeout,
        poll_interval=heartbeat_interval,
    )
    try:
        tunnel_common.wait_for_alive_entry(
            env["server_runner"],
            "server",
            env["server_install_path"],
            env["endpoint_tag"],
            waits.SERVER_IP,
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
            waits.SERVER_IP,
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

