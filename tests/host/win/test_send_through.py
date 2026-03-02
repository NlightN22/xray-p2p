import json
from pathlib import Path

import pytest

from tests.host.win import env as _env

CLIENT_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
CLIENT_CONFIG_DIR_NAME = "config-client"
CLIENT_CONFIG_DIR = _env.CONFIG_ROOT / CLIENT_CONFIG_DIR_NAME
CLIENT_OUTBOUNDS_JSON = CLIENT_CONFIG_DIR / "outbounds.json"
CLIENT_LOG_RELATIVE = r"logs\client.err"
CLIENT_TUN_NAME = "xp2pc"
CLIENT_STATE_FILES = [
    _env.CONFIG_ROOT / "xp2p-client.toml",
    _env.CONFIG_ROOT / "xp2p-client.state.json",
]

SERVER_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
SERVER_CONFIG_DIR_NAME = "config-server"
SERVER_CONFIG_DIR = _env.CONFIG_ROOT / SERVER_CONFIG_DIR_NAME
SERVER_OUTBOUNDS_JSON = SERVER_CONFIG_DIR / "outbounds.json"
SERVER_LOG_RELATIVE = r"logs\server.err"
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




def _read_remote_json(host, path: Path) -> dict:
    content = _env.read_text(host, path)
    try:
        return json.loads(content)
    except json.JSONDecodeError as exc:
        pytest.fail(f"Failed to parse JSON from {path}: {exc}\nContent:\n{content}")


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

        with xp2p_client_run_factory(
            str(CLIENT_INSTALL_DIR), CLIENT_CONFIG_DIR_NAME, CLIENT_LOG_RELATIVE
        ) as session:
            assert session["pid"] > 0
            _assert_tun_interface(client_host, CLIENT_TUN_NAME)
            _assert_dns_resolution(client_host, "2ip.ru")

        outbounds = _read_remote_json(client_host, CLIENT_OUTBOUNDS_JSON)
        direct = _find_outbound(outbounds, "direct")
        expected = _env.get_default_ipv4_sendthrough(client_host)
        actual = _send_through_value(direct)
        if expected:
            assert actual == expected
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

        with xp2p_server_run_factory(
            str(SERVER_INSTALL_DIR), SERVER_CONFIG_DIR_NAME, SERVER_LOG_RELATIVE
        ) as session:
            assert session["pid"] > 0
            _assert_tun_interface(server_host, SERVER_TUN_NAME)
            _assert_dns_resolution(server_host, "2ip.ru")

        outbounds = _read_remote_json(server_host, SERVER_OUTBOUNDS_JSON)
        direct = _find_outbound(outbounds, "direct")
        expected = _env.get_default_ipv4_sendthrough(server_host)
        actual = _send_through_value(direct)
        if expected:
            assert actual == expected
        else:
            assert actual is None
    finally:
        _cleanup_server_install(server_host)
