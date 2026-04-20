from pathlib import Path

import pytest

from tests.host.win import env as _env
from tests.host.win.diagnostics import live_files
from tests.host.win.flows import live_xray

CLIENT_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
CLIENT_CONFIG_DIR_NAME = "config-client"
CLIENT_CONFIG_DIR = _env.CONFIG_ROOT / CLIENT_CONFIG_DIR_NAME
CLIENT_LIVE_XRAY_JSON = _env.CONFIG_LIVE_ROOT / CLIENT_CONFIG_DIR_NAME / "xray.json"
CLIENT_TUN_NAME = "xp2pc"
CLIENT_STATE_FILES = [
    _env.CONFIG_ROOT / "xp2p-client.toml",
    _env.CONFIG_ROOT / "xp2p-client.state.json",
]

SERVER_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
SERVER_CONFIG_DIR_NAME = "config-server"
SERVER_CONFIG_DIR = _env.CONFIG_ROOT / SERVER_CONFIG_DIR_NAME
SERVER_LIVE_XRAY_JSON = _env.CONFIG_LIVE_ROOT / SERVER_CONFIG_DIR_NAME / "xray.json"
SERVER_TUN_NAME = "xp2ps"
SERVER_STATE_FILES = [
    _env.CONFIG_ROOT / "xp2p-server.toml",
    _env.CONFIG_ROOT / "xp2p-server.state.json",
]


def _cleanup_client_install(client_host) -> None:
    _env.cleanup_xp2p_install(
        client_host,
        config_dirs=[CLIENT_CONFIG_DIR],
        state_files=CLIENT_STATE_FILES,
    )


def _cleanup_server_install(server_host) -> None:
    _env.cleanup_xp2p_install(
        server_host,
        config_dirs=[SERVER_CONFIG_DIR],
        state_files=SERVER_STATE_FILES,
    )




def _find_outbound(data: dict, tag: str) -> dict:
    for outbound in data.get("outbounds", []):
        if outbound.get("tag") == tag:
            return outbound
    raise AssertionError(f"Expected outbound with tag {tag} to exist")


def _send_through_value(outbound: dict) -> str | None:
    value = outbound.get("sendThrough")
    if value is None:
        return None
    return str(value)


def _assert_tun_interface(host, interface_name: str) -> None:
    result = _env.run_guest_script(
        host,
        "scripts/assert_tun_interface.ps1",
        InterfaceName=interface_name,
    )
    if result.rc != 0:
        pytest.fail(
            "TUN interface check failed.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def _assert_dns_resolution(host, name: str) -> None:
    result = _env.run_guest_script(
        host,
        "scripts/resolve_dns.ps1",
        Name=name,
    )
    if result.rc != 0:
        pytest.fail(
            "DNS resolution check failed.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


@pytest.mark.host
@pytest.mark.win
def test_client_run_sets_send_through(
    client_host, xp2p_client_runner, xp2p_client_run_factory
):
    _cleanup_client_install(client_host)
    try:
        xp2p_client_runner(
            "client",
            "install",
            "--host",
            "10.62.10.10",
            "--user",
            "sendthrough@example.com",
            "--password",
            "sendthrough-pass",
            "--force",
            check=True,
            )
        live_xray.ensure_live_xray_json(
            client_host,
            xp2p_client_runner,
            role="client",
            xray_path=CLIENT_LIVE_XRAY_JSON,
        )

        with xp2p_client_run_factory(
            str(CLIENT_INSTALL_DIR), CLIENT_CONFIG_DIR_NAME
        ) as session:
            assert session["pid"] > 0
            _assert_tun_interface(client_host, CLIENT_TUN_NAME)
            _assert_dns_resolution(client_host, "2ip.ru")

        xray = live_files.read_json(client_host, CLIENT_LIVE_XRAY_JSON, label="read_live_xray_json")
        direct = _find_outbound(xray, "direct")
        expected = _env.get_default_ipv4_sendthrough(client_host)
        actual = _send_through_value(direct)
        if expected:
            assert actual is None or actual == expected
        else:
            assert actual is None
    finally:
        _cleanup_client_install(client_host)


@pytest.mark.host
@pytest.mark.win
def test_server_run_sets_send_through(
    server_host, xp2p_server_runner, xp2p_server_run_factory
):
    _cleanup_server_install(server_host)
    try:
        xp2p_server_runner(
            "server",
            "install",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR_NAME,
            "--port",
            "62021",
            "--host",
            "sendthrough.test.local",
            "--force",
            check=True,
            )
        live_xray.ensure_live_xray_json(
            server_host,
            xp2p_server_runner,
            role="server",
            xray_path=SERVER_LIVE_XRAY_JSON,
        )

        with xp2p_server_run_factory(
            str(SERVER_INSTALL_DIR), SERVER_CONFIG_DIR_NAME
        ) as session:
            assert session["pid"] > 0
            _assert_tun_interface(server_host, SERVER_TUN_NAME)
            _assert_dns_resolution(server_host, "2ip.ru")

        xray = live_files.read_json(server_host, SERVER_LIVE_XRAY_JSON, label="read_live_xray_json")
        direct_udp = _find_outbound(xray, "direct-udp")
        expected = _env.get_default_ipv4_sendthrough(server_host)
        actual = _send_through_value(direct_udp)
        if expected:
            assert actual is None or actual == expected
        else:
            assert actual is None
        direct_random = _find_outbound(xray, "direct-random")
        assert _send_through_value(direct_random) is None
    finally:
        _cleanup_server_install(server_host)
