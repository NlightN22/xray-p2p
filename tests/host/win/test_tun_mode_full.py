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


def _wait_for_client_apply(
    client_host,
    xp2p_client_runner,
    *,
    ensure_config: bool = False,
    allow_restart: bool = False,
) -> None:
    svc.wait_for_apply_request_clear(client_host)
    if ensure_config:
        svc.wait_for_client_config(client_host)
    svc.wait_for_service_state(xp2p_client_runner, expected_active=True)
    if not allow_restart:
        return
    tail = diag.read_log_tail(client_host, tun.CLIENT_SERVICE_LOG)
    if "deferring route apply until restart" in (tail or "").lower():
        svc.restart_client_service(xp2p_client_runner)
        svc.wait_for_service_state(xp2p_client_runner, expected_active=True)


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
        svc.restart_client_service(xp2p_client_runner)
        svc.wait_for_apply_request_clear(client_host)

        tun_name = tun.client_tun_name(client_host)
        tun_name, _ = tun.wait_for_tun_adapter(client_host, tun_name)
        result = _env.run_guest_script(
            client_host,
            "scripts/wait_for_tcp_listener.ps1",
            Host="127.0.0.1",
            Port="51180",
            TimeoutSeconds="30",
            )
        if result.rc != 0:
            pytest.fail(
                "SOCKS health check failed after client service restart.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
        tail = diag.read_log_tail(client_host, tun.CLIENT_SERVICE_LOG)
        assert "socks health check ok" in (tail or "").lower()
        return
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
