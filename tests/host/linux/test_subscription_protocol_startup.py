from __future__ import annotations

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import _runtime_disable as runtime
from tests.host.linux.test_subscription_control_plane import (
    CONTROL_PORT,
    SERVER_HOST,
    TROJAN_PORT,
    _assert_tunnel_ping,
    _client_outbound_protocol,
    _detect_host_ipv4,
    _extract_link,
    _vless_users,
)

pytestmark = [
    pytest.mark.host,
    pytest.mark.linux,
    pytest.mark.serial,
]

VLESS_USER = "vless-startup@example.com"
VLESS_PASSWORD = "550e8400-e29b-41d4-a716-446655440004"
TROJAN_USER = "trojan-startup@example.com"
TROJAN_PASSWORD = "550e8400-e29b-41d4-a716-446655440005"


def test_01_client_connects_to_vless_profile_from_startup(client_host, server_host):
    _assert_client_connects_from_startup(
        client_host,
        server_host,
        "vless-tls-vision",
        VLESS_USER,
        VLESS_PASSWORD,
        "vless",
    )


def test_02_client_connects_to_trojan_profile_from_startup(client_host, server_host):
    _assert_client_connects_from_startup(
        client_host,
        server_host,
        "trojan-tls",
        TROJAN_USER,
        TROJAN_PASSWORD,
        "trojan",
    )


def _assert_client_connects_from_startup(
    client_host, server_host, profile: str, user: str, password: str, protocol: str
) -> None:
    server_runner = runtime.xp2p_runner(server_host)
    client_runner = runtime.xp2p_runner(client_host)
    try:
        server_ip = _detect_host_ipv4(server_host)
        _add_hosts_entry(client_host, server_ip, SERVER_HOST)
        server_runner(
            "server",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--host",
            SERVER_HOST,
            "--port",
            TROJAN_PORT,
            "--force",
            check=True,
        )
        if profile == "vless-tls-vision":
            server_runner("server", "profile", profile, check=True)
        user_add = server_runner(
            "server",
            "user",
            "add",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.SERVER_CONFIG_DIR_NAME,
            "--id",
            user,
            "--password",
            password,
            "--host",
            SERVER_HOST,
            check=True,
        )
        link = _extract_link(user_add.stdout or "")
        assert link.startswith(f"{protocol}://")
        client_runner(
            "client",
            "install",
            "--path",
            helpers.INSTALL_ROOT.as_posix(),
            "--config-dir",
            helpers.CLIENT_CONFIG_DIR_NAME,
            "--link",
            link,
            "--mode",
            "proxy",
            check=True,
        )
        runtime.start_service(server_host, server_runner, "server")
        runtime.start_service(client_host, client_runner, "client")
        server_live = runtime.wait_for_live_xray(server_host, "server")
        if protocol == "vless":
            assert user in _vless_users(server_live)
        assert _client_outbound_protocol(
            runtime.wait_for_live_xray(client_host, "client"), SERVER_HOST
        ) == protocol
        _assert_tunnel_ping(client_host, server_host, client_runner, SERVER_HOST)
    finally:
        runtime.stop_service(client_runner, "client")
        runtime.stop_service(server_runner, "server")


def _add_hosts_entry(host, ip_address: str, name: str) -> None:
    escaped_name = name.replace("'", "'\\''")
    escaped_ip = ip_address.replace("'", "'\\''")
    host.run(
        "sudo -n /bin/sh -c "
        f"'tmp=$(mktemp); grep -v \"[[:space:]]{escaped_name}$\" /etc/hosts > \"$tmp\"; "
        f"printf \"%s %s\\n\" \"{escaped_ip}\" \"{escaped_name}\" >> \"$tmp\"; "
        "cat \"$tmp\" > /etc/hosts; rm -f \"$tmp\"'"
    )
