from __future__ import annotations

import time

import pytest

from tests.host.win import env as _env
from tests.host.win import tun_full_config as cfg
from tests.host.win import tun_full_diagnostics as diag
from tests.host.win import tun_full_deploy as tun_deploy
from tests.host.win import tun_full_helpers as tun
from tests.host.win import tun_full_internet as net
from tests.host.win import tun_full_service as svc

pytestmark = [pytest.mark.host, pytest.mark.win]

CLIENT_USER = "full-tun@example.com"
CLIENT_PASSWORD = "full-tun-pass"
TROJAN_PORT = "58601"

SERVER_CONFIG_DIR_NAME = "config-server"
SERVER_CONFIG_DIR = _env.CONFIG_ROOT / SERVER_CONFIG_DIR_NAME
SERVER_STATE_FILES = [
    _env.CONFIG_ROOT / "xp2p-server.toml",
    _env.CONFIG_ROOT / "xp2p-server.state.json",
]


def test_windows_client_tun_mode_full_routes(
    client_host,
    server_host,
    server_host_ipv4,
    xp2p_client_runner,
    xp2p_server_runner,
):
    net.ensure_internet_or_skip(server_host, "server")
    net.ensure_internet_or_skip(client_host, "client")

    client_proc = None
    server_proc = None
    service_started = False
    server_service_started = False
    try:
        _env.stop_xp2p_processes(client_host)
        _env.stop_xp2p_processes(server_host)
        xp2p_client_runner("client", "remove", "--all", "--ignore-missing")
        xp2p_server_runner("server", "remove", "--ignore-missing")

        tun.cleanup_client_install(client_host, xp2p_client_runner)
        _env.cleanup_xp2p_install(
            server_host,
            config_dirs=[SERVER_CONFIG_DIR],
            state_files=SERVER_STATE_FILES,
        )
        for host in (client_host, server_host):
            _env.remove_paths(host, tun_deploy.HEARTBEAT_STATE_FILES)
            tun_deploy.remove_deploy_logs(host)

        client_proc, server_proc = tun_deploy.start_deploy_tunnel(
            client_host,
            server_host,
            server_host_ip=server_host_ipv4,
            trojan_user=CLIENT_USER,
            trojan_password=CLIENT_PASSWORD,
            trojan_port=TROJAN_PORT,
        )

        svc.require_client_service(client_host)
        cfg.update_client_config(client_host, tun_mode="split")
        cfg.client_mode(xp2p_client_runner, "tun", "split", check=True)

        tun_deploy.stop_deploy_process(client_host, client_proc)
        tun_deploy.stop_deploy_process(server_host, server_proc)
        client_proc = None
        server_proc = None

        svc.start_service(xp2p_server_runner, "server")
        server_service_started = True
        svc.start_client_service(xp2p_client_runner)
        service_started = True
        svc.wait_for_apply_request_clear(server_host)
        svc.wait_for_apply_request_clear(client_host)

        defaults = tun.default_routes(client_host)
        assert defaults, "Expected at least one default route before full-tunnel"
        old_default_ids = {tun.route_id(route) for route in defaults if tun.route_id(route)[0]}
        assert old_default_ids, "Failed to capture default gateway routes"

        expected_tag = tun.expected_tag(server_host_ipv4)
        result = cfg.client_mode(
            xp2p_client_runner,
            "tun",
            "full",
            "--tag",
            expected_tag,
            check=False,
        )
        if result.rc != 0:
            pytest.fail(
                "xp2p client mode tun full failed.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        client_cfg = _env.read_toml(client_host, tun.CLIENT_CONFIG_FILE).get("client") or {}
        assert client_cfg.get("tun_mode") == "full", "client.tun_mode was not updated to full"
        assert client_cfg.get("full_tunnel_tag") == expected_tag, "client.full_tunnel_tag was not updated"
        svc.wait_for_apply_request_clear(client_host)

        tun_name = tun.client_tun_name(client_host)
        tun_name, tun_index = tun.wait_for_tun_adapter(client_host, tun_name)
        endpoint_ips = [server_host_ipv4]
        ok, debug = tun.poll_for_full_tunnel(
            client_host,
            tun_name,
            tun_index,
            old_default_ids,
            endpoint_ips,
            tun.ROUTE_WAIT_INITIAL,
        )
        if not ok:
            time.sleep(tun.WATCH_RESTART_WINDOW)
            result = cfg.client_mode(
                xp2p_client_runner,
                "tun",
                "full",
                "--tag",
                expected_tag,
                check=False,
            )
            if result.rc != 0:
                pytest.fail(
                    "xp2p client mode tun full retry failed.\n"
                    f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}\n{debug}"
                )
            ok, debug = tun.poll_for_full_tunnel(
                client_host,
                tun_name,
                tun_index,
                old_default_ids,
                endpoint_ips,
                tun.ROUTE_WAIT_RETRY,
            )
            if not ok:
                pytest.fail(debug)

        net.assert_internet_access(client_host, label="full-tunnel enabled")

        result = cfg.client_mode(xp2p_client_runner, "tun", "split", check=False)
        if result.rc != 0:
            pytest.fail(
                "xp2p client mode tun split failed.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        svc.wait_for_apply_request_clear(client_host)
        ok, debug = tun.poll_for_routes_restored(
            client_host,
            tun_name,
            tun_index,
            old_default_ids,
            endpoint_ips,
            tun.ROUTE_WAIT_SPLIT,
        )
        if not ok:
            diag.fail_with_restore_debug(
                client_host,
                tun_name,
                debug,
                config_file=tun.CLIENT_CONFIG_FILE,
                state_file=_env.CONFIG_ROOT / "xp2p-client.tun-full.json",
                service_log=tun.CLIENT_SERVICE_LOG,
            )

        net.assert_internet_access(client_host, label="after split restore")

        result = cfg.client_mode(
            xp2p_client_runner,
            "tun",
            "full",
            "--tag",
            expected_tag,
            check=False,
        )
        if result.rc != 0:
            pytest.fail(
                "xp2p client mode tun full (second pass) failed.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        svc.wait_for_apply_request_clear(client_host)
        ok, debug = tun.poll_for_full_tunnel(
            client_host,
            tun_name,
            tun_index,
            old_default_ids,
            endpoint_ips,
            tun.ROUTE_WAIT_RETRY,
        )
        if not ok:
            pytest.fail(debug)

        net.assert_internet_access(client_host, label="full-tunnel enabled (second pass)")

        svc.stop_client_service(xp2p_client_runner)
        service_started = False
        ok, debug = tun.poll_for_routes_restored(
            client_host,
            tun_name,
            tun_index,
            old_default_ids,
            endpoint_ips,
            tun.ROUTE_WAIT_SPLIT,
        )
        if not ok:
            diag.fail_with_restore_debug(
                client_host,
                tun_name,
                debug,
                config_file=tun.CLIENT_CONFIG_FILE,
                state_file=_env.CONFIG_ROOT / "xp2p-client.tun-full.json",
                service_log=tun.CLIENT_SERVICE_LOG,
            )
        net.assert_internet_access(client_host, label="after service stop")
    finally:
        if client_proc:
            tun_deploy.stop_deploy_process(client_host, client_proc)
        if server_proc:
            tun_deploy.stop_deploy_process(server_host, server_proc)
        if service_started:
            svc.stop_client_service(xp2p_client_runner)
        if server_service_started:
            svc.stop_service(xp2p_server_runner, "server")
        _env.stop_xp2p_processes(client_host)
        _env.stop_xp2p_processes(server_host)
        xp2p_client_runner("client", "remove", "--all", "--ignore-missing")
        xp2p_server_runner("server", "remove", "--ignore-missing")
        tun_deploy.ensure_deploy_firewall_rules(
            server_host,
            trojan_port=TROJAN_PORT,
            ensure="Absent",
        )
        for host in (client_host, server_host):
            _env.remove_paths(host, tun_deploy.HEARTBEAT_STATE_FILES)
            tun_deploy.remove_deploy_logs(host)
