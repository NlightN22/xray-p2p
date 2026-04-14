from __future__ import annotations

from contextlib import contextmanager
import re
import shlex
import time

import pytest

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env
from tests.host.tunnel import common as tunnel_common

SERVER_MACHINE = openwrt_env.OPENWRT_MACHINES[0]
CLIENT_MACHINE = openwrt_env.OPENWRT_MACHINES[1]
CLIENT_TUNNEL_IP = "10.63.30.12"
SERVER_IP = "10.63.30.11"
DIAG_IP = "10.0.200.10"
DIAG_CIDR = f"{DIAG_IP}/32"
DIAG_DOMAIN = "diag.service.internal"
CLIENT_DIAG_IP = "10.0.200.11"
CLIENT_DIAG_CIDR = f"{CLIENT_DIAG_IP}/32"
CLIENT_DIAG_DOMAIN = "diag.client.service"
SOCKS_PORT = 51180
SERVER_DIAGNOSTICS_PORT = 62022
CLIENT_DIAGNOSTICS_PORT = 62023
REVERSE_TUNNEL_WARMUP_SECONDS = 2.5
SERVER_HEARTBEAT_STATE_FILE = helpers.SERVER_HEARTBEAT_STATE_FILE
CLIENT_HEARTBEAT_STATE_FILE = helpers.CLIENT_HEARTBEAT_STATE_FILE
APPLY_REQUEST = helpers.CONFIG_ROOT / helpers.APPLY_DIR_NAME / "apply.request"
REQUIRED_XRAY_CONFIGS = ("inbounds.json", "logs.json", "outbounds.json", "routing.json")


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


def _assert_config_ready(host, role: str) -> None:
    live = helpers.CONFIG_ROOT / f"xp2p-{role}.toml"
    if helpers.path_exists_live(host, live):
        return
    pending_dir = (helpers.CONFIG_ROOT / helpers.APPLY_DIR_NAME / helpers.PENDING_DIR_NAME).as_posix()
    listing = host.run(f"ls -lha {pending_dir} 2>/dev/null || true").stdout or ""
    raise AssertionError(
        f"Missing {role} config for xp2p run on {host.backend.hostname}.\n"
        f"Pending dir listing:\n{listing}"
    )


def _xray_configs_missing(host, config_dir) -> list[str]:
    return [
        (config_dir / name).as_posix()
        for name in REQUIRED_XRAY_CONFIGS
        if not helpers.path_exists_live(host, config_dir / name)
    ]


def _apply_pending_config(host, role: str, install_path: str, config_dir: str) -> None:
    helpers.state_pending_config(host, role)


@contextmanager
def _run_sessions(server_host, client_host):
    _apply_pending_config(
        server_host,
        "server",
        helpers.INSTALL_ROOT.as_posix(),
        helpers.SERVER_CONFIG_DIR_NAME,
    )
    _apply_pending_config(
        client_host,
        "client",
        helpers.INSTALL_ROOT.as_posix(),
        helpers.CLIENT_CONFIG_DIR_NAME,
    )
    helpers.wait_for_live_config(server_host, "server")
    helpers.wait_for_live_config(client_host, "client")
    _assert_config_ready(server_host, "server")
    _assert_config_ready(client_host, "client")
    _ensure_service_running(server_host, "server")
    _ensure_service_running(client_host, "client")
    _wait_for_port(client_host, SOCKS_PORT)
    time.sleep(REVERSE_TUNNEL_WARMUP_SECONDS)
    _wait_for_apply_request_clear(server_host)
    _wait_for_apply_request_clear(client_host)
    yield


def _wait_for_live_config(
    host,
    role: str,
    *,
    timeout_seconds: float = 30.0,
    interval: float = 1.0,
) -> None:
    helpers.wait_for_live_config(
        host,
        role,
        timeout_seconds=timeout_seconds,
        poll_interval=interval,
    )


def _is_xp2p_run_active(host, role: str) -> bool:
    cmd = (
        "ps w | "
        "grep -E "
        + shlex.quote(rf"xp2p {role} (run|service run)")
        + " | grep -v grep >/dev/null 2>&1"
    )
    return host.run(cmd).rc == 0


def _wait_for_service_state(
    host,
    role: str,
    expected_active: bool,
    *,
    timeout_seconds: float = 45.0,
    interval: float = 1.5,
) -> None:
    deadline = time.time() + timeout_seconds
    script = f"/etc/init.d/xp2p-{role}"
    last = None
    while time.time() < deadline:
        result = host.run(f"{script} running")
        active = result.rc == 0
        if active == expected_active:
            return
        last = result
        time.sleep(interval)
    stdout = getattr(last, "stdout", "") or ""
    stderr = getattr(last, "stderr", "") or ""
    state = "active" if expected_active else "inactive"
    raise AssertionError(
        f"xp2p {role} service did not reach {state} state.\nSTDOUT:\n{stdout}\nSTDERR:\n{stderr}"
    )


def _ensure_service_running(host, role: str) -> None:
    if _is_xp2p_run_active(host, role):
        return
    start = host.run(f"/etc/init.d/xp2p-{role} start")
    if start.rc != 0:
        pytest.fail(
            "Failed to start service "
            f"xp2p-{role} on {host.backend.hostname}.\nSTDOUT:\n{start.stdout}\nSTDERR:\n{start.stderr}"
        )
    _wait_for_service_state(host, role, expected_active=True)


def _find_interface_for_ip(host, ip: str) -> str:
    escaped = re.escape(ip)
    command = f"ip -o -4 addr show | awk '$4 ~ /^{escaped}\\// {{print $2; exit}}'"
    result = host.run(command)
    interface = (result.stdout or "").strip().splitlines()
    if not interface:
        pytest.fail(f"Unable to find interface for {ip} on {host.backend.hostname}. STDOUT: {result.stdout}")
    return interface[0]


def _add_ip_alias(host, iface: str, cidr: str) -> None:
    host.run(f"ip addr del {cidr} dev {iface} >/dev/null 2>&1 || true")
    add_result = host.run(f"ip addr add {cidr} dev {iface}")
    if add_result.rc != 0:
        pytest.fail(f"Failed to add IP alias {cidr} on {iface}: {add_result.stdout}\n{add_result.stderr}")


def _remove_ip_alias(host, iface: str, cidr: str) -> None:
    host.run(f"ip addr del {cidr} dev {iface} >/dev/null 2>&1 || true")


def _stop_xp2p_processes(host) -> None:
    openwrt_env._stop_xp2p_services(host)
    host.run("pkill -f 'xp2p server run' >/dev/null 2>&1 || true")
    host.run("pkill -f 'xp2p client run' >/dev/null 2>&1 || true")
    host.run("pkill -f 'xp2p diag' >/dev/null 2>&1 || true")
    host.run("pkill -f 'xp2p' >/dev/null 2>&1 || true")
    host.run("pkill -f '/etc/xp2p/bin/xray' >/dev/null 2>&1 || true")
    host.run("killall xp2p >/dev/null 2>&1 || true")
    host.run("killall -9 xp2p >/dev/null 2>&1 || true")
    host.run("killall xray >/dev/null 2>&1 || true")
    host.run("killall -9 xray >/dev/null 2>&1 || true")
    for port in ("52080", "52180", "51080", "51180"):
        host.run(f"fuser -k {port}/tcp >/dev/null 2>&1 || true")
        host.run(f"fuser -k {port}/udp >/dev/null 2>&1 || true")
    host.run("fuser -k 62022/tcp >/dev/null 2>&1 || true")
    host.run("fuser -k 62022/udp >/dev/null 2>&1 || true")
    host.run("fuser -k 62023/tcp >/dev/null 2>&1 || true")
    host.run("fuser -k 62023/udp >/dev/null 2>&1 || true")


def _add_hosts_entry(host, ip: str, domain: str) -> None:
    host.run(f"sed -i '/{domain}/d' /etc/hosts >/dev/null 2>&1 || true")
    result = host.run(f"echo '{ip} {domain}' >> /etc/hosts")
    if result.rc != 0:
        pytest.fail(f"Failed to append hosts entry {domain} -> {ip} on {host.backend.hostname}")


def _remove_hosts_entry(host, domain: str) -> None:
    host.run(f"sed -i '/{domain}/d' /etc/hosts >/dev/null 2>&1 || true")


def _combined_output(result) -> str:
    return f"{result.stdout}\n{result.stderr}".lower()


def _wait_for_port(host, port: int, *, timeout_seconds: float = 20.0, interval: float = 1.0) -> None:
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        check = host.run(f"netstat -tnl | grep -q ':{port} '")
        if check.rc == 0:
            return
        time.sleep(interval)
    pytest.fail(f"Port {port} did not open on {host.backend.hostname} within {timeout_seconds}s")


def _wait_for_apply_request_clear(host, *, timeout_seconds: float = 30.0, interval: float = 1.0) -> None:
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        if not helpers.path_exists(host, APPLY_REQUEST):
            return
        time.sleep(interval)
    pytest.fail(f"apply.request did not clear within {timeout_seconds}s on {host.backend.hostname}")


def _assert_socks_inbound_listen(host, path, expected_listens: set[str]) -> None:
    data = helpers.read_live_json(host, path)
    for inbound in data.get("inbounds") or []:
        if inbound.get("protocol") != "socks":
            continue
        listen = (inbound.get("listen") or "").strip() or "127.0.0.1"
        if listen in expected_listens:
            return
        raise AssertionError(
            f"SOCKS listen {listen!r} does not match {sorted(expected_listens)} in {path}"
        )
    raise AssertionError(f"SOCKS inbound not found in {path}")


def _assert_port_listen_host(host, port: int, expected_hosts: set[str]) -> None:
    result = host.run("netstat -tnl")
    output = result.stdout or ""
    for host_addr in expected_hosts:
        if f"{host_addr}:{port}" in output:
            return
    pytest.fail(
        f"Expected port {port} to listen on {sorted(expected_hosts)} "
        f"on {host.backend.hostname}. netstat:\n{output}"
    )


def _dump_client_inbounds(host, label: str) -> None:
    paths = [
        helpers.CLIENT_CONFIG_DIR / "inbounds.json",
        helpers.CLIENT_PENDING_DIR / "inbounds.json",
    ]
    print(f"==== CLIENT INBOUNDS ({label}) on {host.backend.hostname} ====")
    for path in paths:
        if not helpers.path_exists(host, path):
            print(f"-- {path} (missing)")
            continue
        content = helpers.read_text(host, path)
        print(f"-- {path}")
        print(content)
    print("==== END CLIENT INBOUNDS ====")


def _dump_config_state(host, label: str) -> None:
    helpers.dump_install_dirs(host, label)
    helpers.dump_apply_dirs(host, label)
    helpers.dump_logs(host, label)


def _warmup_reverse_tunnel():
    # Reverse redirects occasionally drop the very first connection immediately
    # after bringing both xp2p daemons online. Give xray/xp2p a short moment to
    # finish wiring the listener before asserting ping delivery.
    time.sleep(REVERSE_TUNNEL_WARMUP_SECONDS)


def _wait_for_alive_entries(
    server_runner,
    client_runner,
    *,
    install_path: str,
    expected_tag: str,
    expected_host: str,
    expected_user: str,
    expected_client_ip: str,
    timeout_seconds: float = 10.0,
) -> None:
    tunnel_common.wait_for_alive_entry(
        server_runner,
        "server",
        install_path,
        expected_tag,
        expected_host,
        expected_user,
        expected_client_ip,
        timeout_seconds=timeout_seconds,
    )
    tunnel_common.wait_for_alive_entry(
        client_runner,
        "client",
        helpers.INSTALL_ROOT.as_posix(),
        expected_tag,
        expected_host,
        expected_user,
        expected_client_ip,
        timeout_seconds=timeout_seconds,
    )


def _wait_for_server_redirect_apply(
    server_host,
    *,
    target: str,
    outbound_tag: str,
    timeout_seconds: float = 20.0,
    poll_interval: float = 1.0,
) -> None:
    deadline = time.time() + timeout_seconds
    last_error: Exception | None = None
    while time.time() < deadline:
        try:
            state = helpers.read_server_applied_state(server_host)
            helpers.assert_server_redirect_state(state, target, outbound_tag)
            return
        except Exception as exc:
            last_error = exc
        time.sleep(poll_interval)
    raise AssertionError(f"Timed out waiting for server redirect apply: {last_error}")


@pytest.mark.host
@pytest.mark.linux
def test_tunnel_redirect_B_to_A(openwrt_host_factory, xp2p_openwrt_ipk):
    server_host = openwrt_host_factory(SERVER_MACHINE)
    client_host = openwrt_host_factory(CLIENT_MACHINE)
    client_primary_ip = helpers.detect_primary_ipv4(client_host)
    reverse_tag: str | None = None
    endpoint_tag: str | None = None
    for machine, host in ((SERVER_MACHINE, server_host), (CLIENT_MACHINE, client_host)):
        openwrt_env.install_ipk_on_host(host, xp2p_openwrt_ipk)

    server_runner = _runner(server_host)
    client_runner = _runner(client_host)

    def cleanup(iface: str | None = None):
        for host in (server_host, client_host):
            _stop_xp2p_processes(host)
        helpers.cleanup_server_install(server_host, server_runner)
        helpers.cleanup_client_install(client_host, client_runner)
        helpers.remove_path(server_host, SERVER_HEARTBEAT_STATE_FILE)
        helpers.remove_path(client_host, CLIENT_HEARTBEAT_STATE_FILE)
        if iface:
            _remove_ip_alias(server_host, iface, DIAG_CIDR)
        _remove_hosts_entry(server_host, DIAG_DOMAIN)
        if endpoint_tag:
            client_runner(
                "client",
                "redirect",
                "remove",
                "--path",
                helpers.INSTALL_ROOT.as_posix(),
                "--config-dir",
                helpers.CLIENT_CONFIG_DIR_NAME,
                "--cidr",
                DIAG_CIDR,
                "--tag",
                endpoint_tag,
                check=False,
            )

    iface_name = _find_interface_for_ip(server_host, SERVER_IP)
    cleanup(iface_name)
    helpers.dump_install_dirs(server_host, "tunnel redirect B to A after cleanup")
    helpers.dump_install_dirs(client_host, "tunnel redirect B to A after cleanup")
    try:

        server_install = server_runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--host",
            SERVER_IP,
            "--force",
            check=True,
        )
        credential = helpers.extract_trojan_credential(server_install.stdout or "")
        reverse_tag = helpers.expected_reverse_tag(credential["user"], SERVER_IP)
        _apply_pending_config(
            server_host,
            "server",
            helpers.INSTALL_ROOT.as_posix(),
            helpers.SERVER_CONFIG_DIR_NAME,
        )
        _wait_for_live_config(server_host, "server")

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
        _apply_pending_config(
            client_host,
            "client",
            helpers.INSTALL_ROOT.as_posix(),
            helpers.CLIENT_CONFIG_DIR_NAME,
        )
        _wait_for_live_config(client_host, "client")
        helpers.dump_install_dirs(server_host, "tunnel redirect B to A after install")
        helpers.dump_install_dirs(client_host, "tunnel redirect B to A after install")

        client_state = helpers.read_live_client_config(client_host)
        client_routing = helpers.read_live_json(client_host, helpers.CLIENT_CONFIG_DIR / "routing.json")
        endpoint_tag = helpers.expected_proxy_tag(SERVER_IP)
        helpers.assert_client_reverse_artifacts(client_routing, reverse_tag, endpoint_tag)
        helpers.assert_client_reverse_state(
            client_state,
            reverse_tag,
            endpoint_tag=endpoint_tag,
            user=credential["user"],
            host=SERVER_IP,
        )
        helpers.assert_reverse_cli_output_live(
            client_runner,
            "client",
            helpers.INSTALL_ROOT,
            helpers.CLIENT_CONFIG_DIR_NAME,
            reverse_tag,
        )

        try:
            with _run_sessions(server_host, client_host):
                initial_ping = client_runner(
                    "ping",
                    DIAG_IP,
                    "--tunnel",
                    "--count",
                    "3",
                    check=False,
                )
                assert initial_ping.rc != 0

            _add_ip_alias(server_host, iface_name, DIAG_CIDR)

            client_runner(
                "client",
                "redirect",
                "add",
                "--path",
                helpers.INSTALL_ROOT.as_posix(),
                "--config-dir",
                helpers.CLIENT_CONFIG_DIR_NAME,
                "--cidr",
                DIAG_CIDR,
                "--tag",
                endpoint_tag,
                check=True,
            )
            _apply_pending_config(
                client_host,
                "client",
                helpers.INSTALL_ROOT.as_posix(),
                helpers.CLIENT_CONFIG_DIR_NAME,
            )

            with _run_sessions(server_host, client_host):
                _wait_for_port(client_host, SOCKS_PORT)
                _wait_for_port(server_host, SERVER_DIAGNOSTICS_PORT)
                heartbeat_state = helpers.wait_for_heartbeat_state(
                    server_host,
                    path=SERVER_HEARTBEAT_STATE_FILE,
                )
                helpers.assert_heartbeat_entry(
                    heartbeat_state,
                    endpoint_tag,
                    host=SERVER_IP,
                    user=credential["user"],
                    client_ip=client_primary_ip,
                )

                redirected_ping = client_runner(
                    "ping",
                    DIAG_IP,
                    "--tunnel",
                    "--count",
                    "3",
                    check=True,
                )
                tunnel_common.assert_zero_loss(redirected_ping, f"redirected ping to {DIAG_IP}")
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
                DIAG_CIDR,
                "--tag",
                endpoint_tag,
                check=False,
            )

        _add_hosts_entry(server_host, DIAG_IP, DIAG_DOMAIN)
        domain_redirect_added = False
        try:
            client_runner(
                "client",
                "redirect",
                "add",
                "--path",
                helpers.INSTALL_ROOT.as_posix(),
                "--config-dir",
                helpers.CLIENT_CONFIG_DIR_NAME,
                "--domain",
                DIAG_DOMAIN,
                "--tag",
                endpoint_tag,
                check=True,
            )
            domain_redirect_added = True
            _apply_pending_config(
                client_host,
                "client",
                helpers.INSTALL_ROOT.as_posix(),
                helpers.CLIENT_CONFIG_DIR_NAME,
            )

            with _run_sessions(server_host, client_host):
                _wait_for_port(client_host, SOCKS_PORT)
                _wait_for_port(server_host, SERVER_DIAGNOSTICS_PORT)
                heartbeat_state = helpers.wait_for_heartbeat_state(
                    server_host,
                    path=SERVER_HEARTBEAT_STATE_FILE,
                )
                helpers.assert_heartbeat_entry(
                    heartbeat_state,
                    endpoint_tag,
                    host=SERVER_IP,
                    user=credential["user"],
                    client_ip=client_primary_ip,
                )

                redirected_domain = client_runner(
                    "ping",
                    DIAG_DOMAIN,
                    "--tunnel",
                    "--count",
                    "3",
                    check=True,
                )
                tunnel_common.assert_zero_loss(redirected_domain, f"redirected ping to {DIAG_DOMAIN}")
        finally:
            if domain_redirect_added:
                client_runner(
                    "client",
                    "redirect",
                    "remove",
                    "--path",
                    helpers.INSTALL_ROOT.as_posix(),
                    "--config-dir",
                    helpers.CLIENT_CONFIG_DIR_NAME,
                    "--domain",
                    DIAG_DOMAIN,
                    "--tag",
                    endpoint_tag,
                    check=False,
                )
    finally:
        cleanup(iface_name)


@pytest.mark.host
@pytest.mark.linux
def test_tunnel_redirect_A_to_B(openwrt_host_factory, xp2p_openwrt_ipk):
    server_host = openwrt_host_factory(SERVER_MACHINE)
    client_host = openwrt_host_factory(CLIENT_MACHINE)
    reverse_tag: str | None = None
    endpoint_tag: str | None = None
    client_primary_ip: str | None = None
    client_iface = _find_interface_for_ip(client_host, CLIENT_TUNNEL_IP)

    def cleanup():
        for host in (server_host, client_host):
            _stop_xp2p_processes(host)
        helpers.cleanup_server_install(server_host, _runner(server_host))
        helpers.cleanup_client_install(client_host, _runner(client_host))
        helpers.remove_path(server_host, SERVER_HEARTBEAT_STATE_FILE)
        helpers.remove_path(client_host, CLIENT_HEARTBEAT_STATE_FILE)
        _remove_ip_alias(client_host, client_iface, CLIENT_DIAG_CIDR)
        if reverse_tag:
            server_cleanup = _runner(server_host)(
                "server",
                "redirect",
                "remove",
                "--path",
                helpers.INSTALL_ROOT.as_posix(),
                "--config-dir",
                helpers.SERVER_CONFIG_DIR_NAME,
                "--cidr",
                CLIENT_DIAG_CIDR,
                "--tag",
                reverse_tag,
                check=False,
            )
            stderr = _combined_output(server_cleanup)
            if server_cleanup.rc != 0 and "no server redirect rules" not in stderr and "not found" not in stderr:
                pytest.fail(
                    f"Failed to remove redirect {CLIENT_DIAG_CIDR}:\n"
                    f"STDOUT:\n{server_cleanup.stdout}\nSTDERR:\n{server_cleanup.stderr}"
                )

    cleanup()
    helpers.dump_install_dirs(server_host, "tunnel redirect A to B after cleanup")
    helpers.dump_install_dirs(client_host, "tunnel redirect A to B after cleanup")
    for machine, host in ((SERVER_MACHINE, server_host), (CLIENT_MACHINE, client_host)):
        openwrt_env.install_ipk_on_host(host, xp2p_openwrt_ipk)
    helpers.dump_install_dirs(server_host, "tunnel redirect A to B after install")
    helpers.dump_install_dirs(client_host, "tunnel redirect A to B after install")

    server_runner = _runner(server_host)
    client_runner = _runner(client_host)
    try:
        _add_ip_alias(client_host, client_iface, CLIENT_DIAG_CIDR)

        server_install = server_runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--host",
            SERVER_IP,
            "--force",
            check=True,
        )
        credential = helpers.extract_trojan_credential(server_install.stdout or "")
        reverse_tag = helpers.expected_reverse_tag(credential["user"], SERVER_IP)
        _apply_pending_config(
            server_host,
            "server",
            helpers.INSTALL_ROOT.as_posix(),
            helpers.SERVER_CONFIG_DIR_NAME,
        )
        _wait_for_live_config(server_host, "server")

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

        _apply_pending_config(
            client_host,
            "client",
            helpers.INSTALL_ROOT.as_posix(),
            helpers.CLIENT_CONFIG_DIR_NAME,
        )
        _wait_for_live_config(client_host, "client")
        live_client = helpers.CONFIG_ROOT / "xp2p-client.toml"
        if not helpers.path_exists_live(client_host, live_client):
            _dump_config_state(client_host, "tunnel redirect A to B missing live client config")
            raise AssertionError("Missing live xp2p-client.toml after apply")
        client_state = helpers.read_live_client_config(client_host)
        client_routing = helpers.read_live_json(client_host, helpers.CLIENT_CONFIG_DIR / "routing.json")
        endpoint_tag = helpers.expected_proxy_tag(SERVER_IP)
        client_primary_ip = helpers.detect_primary_ipv4(client_host)
        helpers.assert_client_reverse_artifacts(client_routing, reverse_tag, endpoint_tag)
        helpers.assert_client_reverse_state(
            client_state,
            reverse_tag,
            endpoint_tag=endpoint_tag,
            user=credential["user"],
            host=SERVER_IP,
        )
        helpers.assert_reverse_cli_output_live(
            client_runner,
            "client",
            helpers.INSTALL_ROOT,
            helpers.CLIENT_CONFIG_DIR_NAME,
            reverse_tag,
        )

        server_missing = _xray_configs_missing(server_host, helpers.SERVER_CONFIG_DIR)
        if server_missing:
            raise AssertionError(f"Missing server xray configs (live or pending): {server_missing}")

        client_missing = _xray_configs_missing(client_host, helpers.CLIENT_CONFIG_DIR)
        if client_missing:
            raise AssertionError(f"Missing client xray configs (live or pending): {client_missing}")

        inbounds_path = helpers.CLIENT_CONFIG_DIR / "inbounds.json"
        if not helpers.path_exists_live(client_host, inbounds_path):
            raise AssertionError("Missing client inbounds.json in live config")
        client_host.run(f"sed -i 's/127\\.0\\.0\\.1/0.0.0.0/g' {inbounds_path.as_posix()}")
        _apply_pending_config(
            client_host,
            "client",
            helpers.INSTALL_ROOT.as_posix(),
            helpers.CLIENT_CONFIG_DIR_NAME,
        )
        _wait_for_live_config(client_host, "client")

        server_runner(
            "server",
            "redirect",
            "add",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--cidr",
            CLIENT_DIAG_CIDR,
            "--tag",
            reverse_tag,
            check=True,
        )
        _apply_pending_config(
            server_host,
            "server",
            helpers.INSTALL_ROOT.as_posix(),
            helpers.SERVER_CONFIG_DIR_NAME,
        )
        _wait_for_live_config(server_host, "server")

        with _run_sessions(server_host, client_host):
            try:
                _wait_for_port(client_host, SOCKS_PORT)
                _wait_for_port(client_host, CLIENT_DIAGNOSTICS_PORT)
                _wait_for_apply_request_clear(client_host)
                _assert_socks_inbound_listen(
                    client_host,
                    helpers.CLIENT_CONFIG_DIR / "inbounds.json",
                    {CLIENT_TUNNEL_IP, "0.0.0.0", "127.0.0.1"},
                )
                _assert_port_listen_host(client_host, SOCKS_PORT, {CLIENT_TUNNEL_IP, "0.0.0.0", "127.0.0.1"})
                heartbeat_state = helpers.wait_for_heartbeat_state(
                    server_host,
                    path=SERVER_HEARTBEAT_STATE_FILE,
                )
                helpers.assert_heartbeat_entry(
                    heartbeat_state,
                    endpoint_tag,
                    host=SERVER_IP,
                    user=credential["user"],
                    client_ip=client_primary_ip,
                )
                _wait_for_alive_entries(
                    server_runner,
                    client_runner,
                    install_path=helpers.INSTALL_ROOT.as_posix(),
                    expected_tag=endpoint_tag,
                    expected_host=SERVER_IP,
                    expected_user=credential["user"],
                    expected_client_ip=client_primary_ip,
                )
                _warmup_reverse_tunnel()

                redirected_ping = server_runner(
                    "ping",
                    CLIENT_DIAG_IP,
                    "--tunnel",
                    "--port",
                    str(CLIENT_DIAGNOSTICS_PORT),
                    "--count",
                    "3",
                    check=True,
                )
                tunnel_common.assert_zero_loss(redirected_ping, f"redirected ping to {CLIENT_DIAG_IP}")
            except BaseException:
                helpers.dump_logs(client_host, "tunnel redirect A to B client")
                helpers.dump_logs(server_host, "tunnel redirect A to B server")
                _dump_client_inbounds(client_host, "tunnel redirect A to B")
                raise

        _add_hosts_entry(client_host, CLIENT_DIAG_IP, CLIENT_DIAG_DOMAIN)
        server_domain_redirect_added = False
        try:
            server_runner(
                "server",
                "redirect",
                "add",
                "--path",
                helpers.INSTALL_ROOT.as_posix(),
                "--config-dir",
                helpers.SERVER_CONFIG_DIR_NAME,
                "--domain",
                CLIENT_DIAG_DOMAIN,
                "--tag",
                reverse_tag,
                check=True,
            )
            server_domain_redirect_added = True
            _apply_pending_config(
                server_host,
                "server",
                helpers.INSTALL_ROOT.as_posix(),
                helpers.SERVER_CONFIG_DIR_NAME,
            )
            _wait_for_live_config(server_host, "server")

            for host in (server_host, client_host):
                _stop_xp2p_processes(host)

            with _run_sessions(server_host, client_host):
                try:
                    _wait_for_port(client_host, SOCKS_PORT)
                    _wait_for_port(client_host, CLIENT_DIAGNOSTICS_PORT)
                    _wait_for_apply_request_clear(client_host)
                    _assert_socks_inbound_listen(
                        client_host,
                        helpers.CLIENT_CONFIG_DIR / "inbounds.json",
                        {CLIENT_TUNNEL_IP, "0.0.0.0", "127.0.0.1"},
                    )
                    _assert_port_listen_host(
                        client_host, SOCKS_PORT, {CLIENT_TUNNEL_IP, "0.0.0.0", "127.0.0.1"}
                    )
                    heartbeat_state = helpers.wait_for_heartbeat_state(
                        server_host,
                        path=SERVER_HEARTBEAT_STATE_FILE,
                    )
                    helpers.assert_heartbeat_entry(
                        heartbeat_state,
                        endpoint_tag,
                        host=SERVER_IP,
                        user=credential["user"],
                        client_ip=client_primary_ip,
                    )
                    _wait_for_alive_entries(
                        server_runner,
                        client_runner,
                        install_path=helpers.INSTALL_ROOT.as_posix(),
                        expected_tag=endpoint_tag,
                        expected_host=SERVER_IP,
                        expected_user=credential["user"],
                        expected_client_ip=client_primary_ip,
                    )
                    _wait_for_server_redirect_apply(
                        server_host,
                        target=CLIENT_DIAG_DOMAIN,
                        outbound_tag=reverse_tag,
                    )
                    _warmup_reverse_tunnel()

                    redirected_domain = server_runner(
                        "ping",
                        CLIENT_DIAG_DOMAIN,
                        "--tunnel",
                        "--port",
                        str(CLIENT_DIAGNOSTICS_PORT),
                        "--count",
                        "3",
                        check=True,
                    )
                    tunnel_common.assert_zero_loss(
                        redirected_domain, f"redirected ping to {CLIENT_DIAG_DOMAIN}"
                    )
                except BaseException:
                    helpers.dump_logs(client_host, "tunnel redirect A to B domain client")
                    helpers.dump_logs(server_host, "tunnel redirect A to B domain server")
                    _dump_client_inbounds(client_host, "tunnel redirect A to B domain")
                    raise
        finally:
            if server_domain_redirect_added:
                server_runner(
                    "server",
                    "redirect",
                    "remove",
                    "--path",
                    helpers.INSTALL_ROOT.as_posix(),
                    "--config-dir",
                    helpers.SERVER_CONFIG_DIR_NAME,
                    "--domain",
                    CLIENT_DIAG_DOMAIN,
                    "--tag",
                    reverse_tag,
                    check=False,
                )
            _remove_hosts_entry(client_host, CLIENT_DIAG_DOMAIN)
    finally:
        cleanup()
