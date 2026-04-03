import json
import time
from pathlib import Path

import pytest

from tests.host.win import env as _env

CLIENT_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
CLIENT_CONFIG_DIR_NAME = "config-client"
CLIENT_CONFIG_DIR = _env.CONFIG_ROOT / CLIENT_CONFIG_DIR_NAME
CLIENT_OUTBOUNDS_JSON = CLIENT_CONFIG_DIR / "outbounds.json"
CLIENT_TUN_NAME = "xp2pc"
CLIENT_STATE_FILES = [
    _env.CONFIG_ROOT / "xp2p-client.toml",
    _env.CONFIG_ROOT / "xp2p-client.state.json",
]

SERVER_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
SERVER_CONFIG_DIR_NAME = "config-server"
SERVER_CONFIG_DIR = _env.CONFIG_ROOT / SERVER_CONFIG_DIR_NAME
SERVER_OUTBOUNDS_JSON = SERVER_CONFIG_DIR / "outbounds.json"
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




def _path_exists_exact(host, path: Path) -> bool:
    quoted = _env.ps_quote(str(path))
    result = _env.run_powershell(
        host,
        f"if (Test-Path {quoted}) {{ exit 0 }} else {{ exit 3 }}",
        label="path_exists_exact",
    )
    return result.rc == 0


def _read_json_exact(host, path: Path) -> dict:
    quoted = _env.ps_quote(str(path))
    script = f"""
$ErrorActionPreference = 'Stop'
if (-not (Test-Path {quoted})) {{
    exit 3
}}
Get-Content -Path {quoted} -Raw
exit 0
"""
    result = _env.run_powershell(host, script, label="read_json_exact")
    if result.rc != 0:
        pytest.fail(
            f"Failed to read JSON {path}.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    try:
        return json.loads(result.stdout)
    except json.JSONDecodeError as exc:
        pytest.fail(f"Failed to parse JSON from {path}: {exc}\nContent:\n{result.stdout}")


def _read_outbounds_json(host, path: Path, *, timeout: float = 30.0) -> dict:
    pending = _env.pending_candidate(path)
    deadline = time.time() + timeout
    while time.time() < deadline:
        if _path_exists_exact(host, path):
            return _read_json_exact(host, path)
        time.sleep(1.0)
    if _path_exists_exact(host, pending):
        return _read_json_exact(host, pending)
    pytest.fail(f"Timed out waiting for outbounds JSON at {path}.")


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


def _wait_for_apply_request_clear(host, *, timeout: float = 60.0) -> None:
    apply_path = _env.CONFIG_ROOT / _env.APPLY_DIR_NAME / "apply.request"
    deadline = time.time() + timeout
    while time.time() < deadline:
        if not _env.path_exists(host, apply_path):
            return
        time.sleep(1.0)
    pytest.fail(f"apply.request did not clear after {timeout} seconds.")


def _ensure_live_outbounds(host, runner, role: str, outbounds_path: Path) -> None:
    if _path_exists_exact(host, outbounds_path):
        return
    service_name = f"xp2p-{role}"
    if not _env.service_exists(host, service_name):
        pytest.skip(f"{service_name} service is not registered; MSI install required.")
    runner(role, "service", "start", check=True)
    _wait_for_apply_request_clear(host, timeout=90.0)
    deadline = time.time() + 30.0
    while time.time() < deadline:
        if _path_exists_exact(host, outbounds_path):
            break
        time.sleep(1.0)
    runner(role, "service", "stop", check=True)
    if not _path_exists_exact(host, outbounds_path):
        pytest.fail(f"Live outbounds.json was not created at {outbounds_path}.")

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
        _ensure_live_outbounds(
            client_host, xp2p_client_runner, "client", CLIENT_OUTBOUNDS_JSON
        )

        with xp2p_client_run_factory(
            str(CLIENT_INSTALL_DIR), CLIENT_CONFIG_DIR_NAME
        ) as session:
            assert session["pid"] > 0
            _assert_tun_interface(client_host, CLIENT_TUN_NAME)
            _assert_dns_resolution(client_host, "2ip.ru")

        outbounds = _read_outbounds_json(client_host, CLIENT_OUTBOUNDS_JSON)
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
        _ensure_live_outbounds(
            server_host, xp2p_server_runner, "server", SERVER_OUTBOUNDS_JSON
        )

        with xp2p_server_run_factory(
            str(SERVER_INSTALL_DIR), SERVER_CONFIG_DIR_NAME
        ) as session:
            assert session["pid"] > 0
            _assert_tun_interface(server_host, SERVER_TUN_NAME)
            _assert_dns_resolution(server_host, "2ip.ru")

        outbounds = _read_outbounds_json(server_host, SERVER_OUTBOUNDS_JSON)
        direct_udp = _find_outbound(outbounds, "direct-udp")
        expected = _env.get_default_ipv4_sendthrough(server_host)
        actual = _send_through_value(direct_udp)
        if expected:
            assert actual == expected
        else:
            assert actual is None
        direct_random = _find_outbound(outbounds, "direct-random")
        assert _send_through_value(direct_random) is None
    finally:
        _cleanup_server_install(server_host)
