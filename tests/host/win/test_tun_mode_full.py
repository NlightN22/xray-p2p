from __future__ import annotations

import json
import time
from pathlib import Path

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
SYNCED_LOG_ROOT = _env.GUEST_BUILD_ROOT / "logs" / "tests"
SYNCED_SERVICE_LOG = SYNCED_LOG_ROOT / "client" / "service.log"
SYNCED_SERVICE_KEEP_LOG = SYNCED_LOG_ROOT / "client" / "service.keep.log"

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
    log_path: Path | None = None,
) -> None:
    svc.wait_for_apply_request_clear(client_host)
    if ensure_config:
        svc.wait_for_client_config(client_host)
    svc.wait_for_service_outcome(client_host, log_path=log_path)
    if not allow_restart:
        return
    tail = diag.read_log_tail(client_host, log_path or tun.CLIENT_SERVICE_LOG)
    if "deferring route apply until restart" in (tail or "").lower():
        xp2p_client_runner("client", "service", "restart", check=True)
        svc.wait_for_service_outcome(client_host, log_path=log_path)


def _wait_for_full_tunnel_state(
    client_host,
    *,
    expected_enabled: bool,
    expected_mode: str,
    timeout: float = 60.0,
) -> dict:
    deadline = time.time() + timeout
    last_state: dict = {}
    while time.time() < deadline:
        state = tun._read_full_tunnel_state(client_host)
        last_state = state
        if not state:
            time.sleep(tun.ROUTE_POLL_INTERVAL)
            continue
        enabled = bool(state.get("enabled"))
        mode = str(state.get("tun_mode") or "").strip().lower()
        if enabled == expected_enabled and mode == expected_mode:
            return state
        time.sleep(tun.ROUTE_POLL_INTERVAL)
    pytest.fail(
        "xp2p-client.tun-full.json did not reach expected state.\n"
        f"Expected: enabled={expected_enabled}, mode={expected_mode}. Last state: {last_state}"
    )


def _preserve_service_log(client_host) -> None:
    script = (
        "$ErrorActionPreference = 'SilentlyContinue'\n"
        f"$src = {_env.ps_quote(str(SYNCED_SERVICE_LOG))}\n"
        f"$dst = {_env.ps_quote(str(SYNCED_SERVICE_KEEP_LOG))}\n"
        "$dstDir = Split-Path -Parent $dst\n"
        "if (Test-Path $src) {\n"
        "    New-Item -ItemType Directory -Path $dstDir -Force | Out-Null\n"
        "    Copy-Item -Path $src -Destination $dst -Force\n"
        "}\n"
    )
    _env.run_powershell(client_host, script, label="preserve_service_log")


def _is_tun_default_route(route: dict, tun_name: str) -> bool:
    alias = str(route.get("InterfaceAlias") or "").strip().lower()
    if not alias:
        return False
    tun_name = tun_name.strip().lower()
    if tun_name:
        return alias == tun_name or alias.startswith(tun_name) or "xray tunnel" in alias or "xp2p" in alias
    return "xray tunnel" in alias or "xp2p" in alias


def _wait_for_non_tun_default_routes(client_host, tun_name: str, timeout: float = 60.0) -> None:
    deadline = time.time() + timeout
    last_defaults: list[dict] = []
    while time.time() < deadline:
        defaults = tun.default_routes(client_host)
        last_defaults = defaults
        if defaults and not any(_is_tun_default_route(route, tun_name) for route in defaults):
            return
        time.sleep(tun.ROUTE_POLL_INTERVAL)
    pytest.fail(f"Default routes still point to tun after {timeout:.0f}s: {last_defaults}")


def _ensure_default_routes_restored(client_host, xp2p_client_runner) -> None:
    state_path = _env.CONFIG_ROOT / "xp2p-client.tun-full.json"
    state_enabled = False
    if _env.path_exists(client_host, state_path):
        try:
            payload = _env.read_text(client_host, state_path)
            state_enabled = bool(json.loads(payload or "{}").get("enabled"))
        except json.JSONDecodeError:
            state_enabled = False
    defaults = tun.default_routes(client_host)
    tun_default = any(_is_tun_default_route(route, "") for route in defaults)
    if not state_enabled and not tun_default:
        return
    xp2p_client_runner("client", "service", "stop", check=False)
    _wait_for_non_tun_default_routes(client_host, "")


def _skip_if_not_direct_file(request) -> None:
    args = [str(arg).replace("\\", "/").lower() for arg in request.config.args]
    if any(arg.endswith("tests/host/win/test_tun_mode_full.py") or arg.endswith("test_tun_mode_full.py") for arg in args):
        return
    pytest.skip("Long-running full-tunnel test; run this file directly to execute.")


def test_windows_client_tun_mode_full_routes(
    client_host,
    server_host,
    server_host_ipv4,
    xp2p_client_runner,
    xp2p_server_runner,
    request,
):
    _skip_if_not_direct_file(request)
    _ensure_default_routes_restored(client_host, xp2p_client_runner)
    net.assert_internet_access(server_host, label="pre-check-server")
    net.assert_internet_access(client_host, label="pre-check-client")

    client_proc = None
    server_proc = None
    service_started = False
    server_service_started = False
    original_env: list[str] | None = None
    success = False
    client_removed = False
    old_default_ids: set[tuple[str, int]] | None = None
    tun_name: str | None = None
    tun_index: int | None = None
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

        _env.run_powershell(
            client_host,
            f"New-Item -ItemType Directory -Path {_env.ps_quote(str(SYNCED_LOG_ROOT))} -Force | Out-Null",
            label="ensure_synced_log_root",
        )
        original_env = svc.set_service_log_root(client_host, SYNCED_LOG_ROOT)
        _env.remove_path(client_host, SYNCED_SERVICE_LOG)

        client_proc, server_proc = tun_deploy.start_deploy_tunnel(
            client_host,
            server_host,
            server_host_ip=server_host_ipv4,
            trojan_user=CLIENT_USER,
            trojan_password=CLIENT_PASSWORD,
            trojan_port=TROJAN_PORT,
            )

        svc.require_client_service(client_host)

        tun_deploy.stop_deploy_process(client_host, client_proc)
        tun_deploy.stop_deploy_process(server_host, server_proc)
        client_proc = None
        server_proc = None

        net.assert_internet_access(client_host, label="pre-client-start")
        net.assert_dns_resolution(client_host, "example.com", label="pre-client-start")
        net.assert_direct_ping(client_host, server_host_ipv4, label="pre-client-start")
        old_default_ids = {tun.route_id(route) for route in tun.default_routes(client_host)}

        svc.start_service(xp2p_server_runner, "server")
        server_service_started = True
        xp2p_client_runner("client", "service", "start", check=True)
        service_started = True
        svc.wait_for_apply_request_clear(server_host)
        svc.wait_for_service_outcome(client_host, log_path=SYNCED_SERVICE_LOG)
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
        svc.wait_for_apply_request_set(client_host)
        _wait_for_client_apply(
            client_host,
            xp2p_client_runner,
            ensure_config=True,
            allow_restart=True,
            log_path=SYNCED_SERVICE_LOG,
        )
        _wait_for_full_tunnel_state(
            client_host,
            expected_enabled=True,
            expected_mode="full",
        )
        client_cfg = _env.read_toml(client_host, tun.CLIENT_CONFIG_FILE).get("client") or {}
        assert client_cfg.get("full_tunnel_tag") == expected_tag, "client.full_tunnel_tag was not updated"
        net.assert_internet_access(client_host, label="full-tun")

        tun_name = tun.client_tun_name(client_host)
        tun_name, tun_index = tun.wait_for_tun_adapter(client_host, tun_name)
        ok, debug = tun.poll_for_full_tunnel(
            client_host,
            tun_name,
            tun_index,
            old_default_ids or set(),
            [server_host_ipv4],
            timeout=tun.ROUTE_WAIT_SPLIT,
            log_path=SYNCED_SERVICE_LOG,
        )
        if not ok:
            diag.fail_with_restore_debug(
                client_host,
                tun_name,
                debug,
                config_file=tun.CLIENT_CONFIG_FILE,
                state_file=_env.CONFIG_ROOT / "xp2p-client.tun-full.json",
                service_log=SYNCED_SERVICE_LOG,
            )

        result = cfg.client_mode(xp2p_client_runner, "tun", "split", check=False)
        if result.rc != 0:
            pytest.fail(
                "xp2p client mode tun split failed.\n"
                f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
            )
        svc.wait_for_apply_request_set(client_host)
        _wait_for_client_apply(
            client_host,
            xp2p_client_runner,
            ensure_config=True,
            allow_restart=True,
            log_path=SYNCED_SERVICE_LOG,
        )
        net.assert_internet_access(client_host, label="post-split")
        ok, debug = tun.poll_for_routes_restored(
            client_host,
            tun_name,
            tun_index,
            old_default_ids or set(),
            [server_host_ipv4],
            timeout=tun.ROUTE_WAIT_SPLIT,
            log_path=SYNCED_SERVICE_LOG,
        )
        if not ok:
            diag.fail_with_restore_debug(
                client_host,
                tun_name,
                debug,
                config_file=tun.CLIENT_CONFIG_FILE,
                state_file=_env.CONFIG_ROOT / "xp2p-client.tun-full.json",
                service_log=SYNCED_SERVICE_LOG,
            )

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
        svc.wait_for_apply_request_set(client_host)
        _wait_for_client_apply(
            client_host,
            xp2p_client_runner,
            ensure_config=True,
            allow_restart=True,
            log_path=SYNCED_SERVICE_LOG,
        )
        net.assert_internet_access(client_host, label="post-full")
        ok, debug = tun.poll_for_full_tunnel(
            client_host,
            tun_name,
            tun_index,
            old_default_ids or set(),
            [server_host_ipv4],
            timeout=tun.ROUTE_WAIT_SPLIT,
            log_path=SYNCED_SERVICE_LOG,
        )
        if not ok:
            diag.fail_with_restore_debug(
                client_host,
                tun_name,
                debug,
                config_file=tun.CLIENT_CONFIG_FILE,
                state_file=_env.CONFIG_ROOT / "xp2p-client.tun-full.json",
                service_log=SYNCED_SERVICE_LOG,
            )

        svc.stop_client_service(xp2p_client_runner)
        service_started = False
        ok, debug = tun.poll_for_routes_restored(
            client_host,
            tun_name,
            tun_index,
            old_default_ids or set(),
            [server_host_ipv4],
            timeout=tun.ROUTE_WAIT_SPLIT,
            log_path=SYNCED_SERVICE_LOG,
        )
        if not ok:
            diag.fail_with_restore_debug(
                client_host,
                tun_name,
                debug,
                config_file=tun.CLIENT_CONFIG_FILE,
                state_file=_env.CONFIG_ROOT / "xp2p-client.tun-full.json",
                service_log=SYNCED_SERVICE_LOG,
            )
        net.assert_internet_access(client_host, label="post-stop")

        xp2p_client_runner("client", "service", "start", check=True)
        service_started = True
        _wait_for_client_apply(
            client_host,
            xp2p_client_runner,
            ensure_config=True,
            allow_restart=True,
            log_path=SYNCED_SERVICE_LOG,
        )
        net.assert_internet_access(client_host, label="post-restart")
        ok, debug = tun.poll_for_full_tunnel(
            client_host,
            tun_name,
            tun_index,
            old_default_ids or set(),
            [server_host_ipv4],
            timeout=tun.ROUTE_WAIT_SPLIT,
            log_path=SYNCED_SERVICE_LOG,
        )
        if not ok:
            diag.fail_with_restore_debug(
                client_host,
                tun_name,
                debug,
                config_file=tun.CLIENT_CONFIG_FILE,
                state_file=_env.CONFIG_ROOT / "xp2p-client.tun-full.json",
                service_log=SYNCED_SERVICE_LOG,
            )

        xp2p_client_runner("client", "remove", "--all", "--ignore-missing")
        client_removed = True
        service_started = False
        ok, debug = tun.poll_for_routes_restored(
            client_host,
            tun_name,
            tun_index,
            old_default_ids or set(),
            [server_host_ipv4],
            timeout=tun.ROUTE_WAIT_SPLIT,
            log_path=SYNCED_SERVICE_LOG,
        )
        if not ok:
            diag.fail_with_restore_debug(
                client_host,
                tun_name,
                debug,
                config_file=tun.CLIENT_CONFIG_FILE,
                state_file=_env.CONFIG_ROOT / "xp2p-client.tun-full.json",
                service_log=SYNCED_SERVICE_LOG,
            )
        net.assert_internet_access(client_host, label="post-remove")

        net.assert_internet_access(client_host, label="post-test")
        success = True
        return
    finally:
        _preserve_service_log(client_host)
        if original_env is not None:
            svc.restore_service_env(client_host, original_env)
        if client_proc:
            tun_deploy.stop_deploy_process(client_host, client_proc)
        if server_proc:
            tun_deploy.stop_deploy_process(server_host, server_proc)
        if service_started:
            svc.stop_client_service(xp2p_client_runner)
            if success and tun_name and tun_index is not None and old_default_ids is not None:
                ok, debug = tun.poll_for_routes_restored(
                    client_host,
                    tun_name,
                    tun_index,
                    old_default_ids,
                    [server_host_ipv4],
                    timeout=tun.ROUTE_WAIT_SPLIT,
                    log_path=SYNCED_SERVICE_LOG,
                )
                if not ok:
                    diag.fail_with_restore_debug(
                        client_host,
                        tun_name,
                        debug,
                        config_file=tun.CLIENT_CONFIG_FILE,
                        state_file=_env.CONFIG_ROOT / "xp2p-client.tun-full.json",
                        service_log=SYNCED_SERVICE_LOG,
                    )
        if server_service_started:
            svc.stop_service(xp2p_server_runner, "server")
        _env.stop_xp2p_processes(client_host)
        _env.stop_xp2p_processes(server_host)
        if not client_removed:
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
        _env.remove_path(client_host, SYNCED_SERVICE_LOG)
