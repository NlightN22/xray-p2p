from __future__ import annotations

import re
import time

import pytest

from tests.host.openwrt import _helpers as helpers
from tests.host.openwrt import env as openwrt_env
from tests.host.tunnel import common as tunnel_common

SERVER_MACHINE = openwrt_env.OPENWRT_MACHINES[0]
CLIENT_MACHINE = openwrt_env.OPENWRT_MACHINES[1]
SERVER_IP = "10.63.30.11"
DIAG_IP = "10.0.200.10"
DIAG_CIDR = f"{DIAG_IP}/32"
SOCKS_PORT = 51180
HEARTBEAT_STATE_FILE = helpers.INSTALL_ROOT / "state-heartbeat.json"


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


@pytest.mark.host
@pytest.mark.linux
def test_tunnel_redirect_B_to_A(openwrt_host_factory, xp2p_openwrt_ipk):
    server_host = openwrt_host_factory(SERVER_MACHINE)
    client_host = openwrt_host_factory(CLIENT_MACHINE)
    client_primary_ip = helpers.detect_primary_ipv4(client_host)
    reverse_tag: str | None = None
    endpoint_tag: str | None = None
    for machine, host in ((SERVER_MACHINE, server_host), (CLIENT_MACHINE, client_host)):
        openwrt_env.sync_build_output(machine)
        openwrt_env.install_ipk_on_host(host, xp2p_openwrt_ipk)

    server_runner = _runner(server_host)
    client_runner = _runner(client_host)

    def cleanup(iface: str | None = None):
        for host in (server_host, client_host):
            host.run("pkill -f 'xp2p server run' >/dev/null 2>&1 || true")
            host.run("pkill -f 'xp2p client run' >/dev/null 2>&1 || true")
            host.run("pkill -f '/etc/xp2p/bin/xray' >/dev/null 2>&1 || true")
        helpers.cleanup_server_install(server_host, server_runner)
        helpers.cleanup_client_install(client_host, client_runner)
        for host in (server_host, client_host):
            helpers.remove_path(host, HEARTBEAT_STATE_FILE)
        if iface:
            _remove_ip_alias(server_host, iface, DIAG_CIDR)
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
    try:
        _add_ip_alias(server_host, iface_name, DIAG_CIDR)

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
            client_runner,
            "client",
            helpers.INSTALL_ROOT,
            helpers.CLIENT_CONFIG_DIR_NAME,
            reverse_tag,
        )

        try:
            initial_ping = client_runner(
                "ping",
                DIAG_IP,
                "--socks",
                "--count",
                "3",
                check=False,
            )
            assert initial_ping.rc != 0

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

            with openwrt_env.xp2p_run_session(
                server_host,
                "server",
                helpers.INSTALL_ROOT.as_posix(),
                helpers.SERVER_CONFIG_DIR_NAME,
                helpers.SERVER_LOG_FILE,
            ), openwrt_env.xp2p_run_session(
                client_host,
                "client",
                helpers.INSTALL_ROOT.as_posix(),
                helpers.CLIENT_CONFIG_DIR_NAME,
                helpers.CLIENT_LOG_FILE,
            ):
                _wait_for_port(client_host, SOCKS_PORT)
                heartbeat_state = helpers.wait_for_heartbeat_state(server_host)
                helpers.assert_heartbeat_entry(
                    heartbeat_state,
                    endpoint_tag,
                    host=SERVER_IP,
                    user=credential["user"],
                    client_ip=client_primary_ip,
                )

                redirected_ping = tunnel_common.ping_with_retries(
                    client_runner,
                    (
                        "ping",
                        DIAG_IP,
                        "--socks",
                        "--count",
                        "3",
                    ),
                    f"redirected ping to {DIAG_IP}",
                    attempts=8,
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
    finally:
        cleanup(iface_name)
