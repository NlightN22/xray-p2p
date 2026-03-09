from __future__ import annotations

import time
from pathlib import Path

import pytest

from tests.host.win import env as _env

INSTALL_DIR = Path(r"C:\Program Files\xp2p")
CLIENT_CONFIG_DIR = "config-client"
SERVER_CONFIG_DIR = "config-server"
CLIENT_LOG_RELATIVE = r"logs\client.err"
SERVER_LOG_RELATIVE = r"logs\server.err"
CLIENT_TUN = "xp2pc"
SERVER_TUN = "xp2ps"

CLIENT_REDIRECT_CIDR = "10.88.0.1/32"
CLIENT_REDIRECT_CIDR_ALT = "10.88.0.2/32"
SERVER_REDIRECT_CIDR = "10.88.1.1/32"

SERVER_STATE_FILES = [
    _env.CONFIG_ROOT / "xp2p-server.toml",
    _env.CONFIG_ROOT / "xp2p-server.state.json",
]
CLIENT_STATE_FILES = [
    _env.CONFIG_ROOT / "xp2p-client.toml",
    _env.CONFIG_ROOT / "xp2p-client.state.json",
]

ROUTE_WAIT_TIMEOUT = 20.0
ROUTE_POLL_INTERVAL = 1.0


def _server_public_host() -> str:
    return _env.DEFAULT_TARGET


def _cleanup_server_install(server_host, runner) -> None:
    _env.run_guest_script(server_host, "scripts/kill_xp2p_processes.ps1")
    runner("server", "remove", "--ignore-missing")
    _env.cleanup_xp2p_install(
        server_host,
        config_dirs=[_env.CONFIG_ROOT / SERVER_CONFIG_DIR],
        state_files=SERVER_STATE_FILES,
    )


def _cleanup_client_install(client_host, runner) -> None:
    _env.run_guest_script(client_host, "scripts/kill_xp2p_processes.ps1")
    runner("client", "remove", "--all", "--ignore-missing")
    _env.cleanup_xp2p_install(
        client_host,
        config_dirs=[_env.CONFIG_ROOT / CLIENT_CONFIG_DIR],
        state_files=CLIENT_STATE_FILES,
    )


def _extract_generated_credential(stdout: str) -> dict[str, str | None]:
    user = password = link = None
    for raw_line in (stdout or "").splitlines():
        line = raw_line.strip()
        lowered = line.lower()
        if lowered.startswith("user:"):
            user = line.split(":", 1)[1].strip()
        elif lowered.startswith("password:"):
            password = line.split(":", 1)[1].strip()
        elif lowered.startswith("link:"):
            link = line.split(":", 1)[1].strip()
    if user is None or password is None:
        pytest.fail(
            "xp2p server install did not emit trojan credential (missing user/password lines).\n"
            f"STDOUT:\n{stdout}"
        )
    return {"user": user, "password": password, "link": link}


def _sanitize_label(value: str) -> str:
    cleaned = value.strip().lower()
    result: list[str] = []
    last_dash = False
    for char in cleaned:
        if char.isalnum():
            result.append(char)
            last_dash = False
            continue
        if char == "-" and not last_dash:
            result.append("-")
            last_dash = True
            continue
        if not last_dash:
            result.append("-")
            last_dash = True
    return "".join(result).strip("-")


def _expected_reverse_tag(user: str, host: str) -> str:
    user_label = _sanitize_label(user)
    host_label = _sanitize_label(host)
    if not user_label or not host_label:
        pytest.fail(f"Unable to derive reverse tag for user={user!r} host={host!r}")
    return f"{user_label}{host_label}.rev"


def _set_client_mode(runner, mode: str) -> None:
    runner(
        "client",
        "mode",
        mode,
        "--path",
        str(INSTALL_DIR),
        "--config-dir",
        CLIENT_CONFIG_DIR,
        check=True,
    )


def _current_client_mode(host) -> str:
    state = _env.read_toml(host, _env.CONFIG_ROOT / "xp2p-client.toml").get("client") or {}
    tun_enabled = state.get("tun_enabled")
    if not isinstance(tun_enabled, bool):
        pytest.fail(f"Expected tun_enabled boolean in client config, got {tun_enabled!r}")
    return "tun" if tun_enabled else "proxy"


def _wait_for_interface_index(host, name: str) -> int:
    deadline = time.time() + ROUTE_WAIT_TIMEOUT
    last_error: Exception | None = None
    while time.time() < deadline:
        try:
            return _env.get_interface_index(host, name)
        except Exception as exc:
            last_error = exc
        time.sleep(ROUTE_POLL_INTERVAL)
    pytest.fail(f"Interface {name} not available: {last_error}")


def _route_snapshot(host, cidr: str) -> list[dict]:
    try:
        return _env.get_net_routes(host, cidr)
    except Exception as exc:
        pytest.fail(f"Failed to read routes for {cidr}: {exc}")


def _wait_for_route_present(host, cidr: str, interface_index: int) -> dict:
    deadline = time.time() + ROUTE_WAIT_TIMEOUT
    last_routes: list[dict] = []
    while time.time() < deadline:
        routes = _route_snapshot(host, cidr)
        last_routes = routes
        if len(routes) != 1:
            time.sleep(ROUTE_POLL_INTERVAL)
            continue
        route = routes[0]
        try:
            route_index = int(route.get("InterfaceIndex"))
        except (TypeError, ValueError):
            time.sleep(ROUTE_POLL_INTERVAL)
            continue
        if route_index != interface_index:
            time.sleep(ROUTE_POLL_INTERVAL)
            continue
        next_hop = (route.get("NextHop") or "").strip()
        if next_hop != "0.0.0.0":
            time.sleep(ROUTE_POLL_INTERVAL)
            continue
        return route
    pytest.fail(f"Timed out waiting for route {cidr}. Last routes: {last_routes}")


def _wait_for_route_absent(host, cidr: str) -> None:
    deadline = time.time() + ROUTE_WAIT_TIMEOUT
    last_routes: list[dict] = []
    while time.time() < deadline:
        routes = _route_snapshot(host, cidr)
        last_routes = routes
        if not routes:
            return
        time.sleep(ROUTE_POLL_INTERVAL)
    pytest.fail(f"Timed out waiting for route {cidr} removal. Last routes: {last_routes}")


@pytest.mark.host
@pytest.mark.win
def test_windows_client_redirect_routes_os(
    server_host,
    client_host,
    xp2p_server_runner,
    xp2p_client_runner,
    xp2p_server_run_factory,
    xp2p_client_run_factory,
):
    _cleanup_server_install(server_host, xp2p_server_runner)
    _cleanup_client_install(client_host, xp2p_client_runner)
    server_public_host = _server_public_host()
    try:
        server_install = xp2p_server_runner(
            "server",
            "install",
            "--host",
            server_public_host,
            "--force",
            check=True,
        )
        credential = _extract_generated_credential(server_install.stdout or "")
        assert credential["link"], "Expected trojan link in server install output"

        xp2p_client_runner(
            "client",
            "install",
            "--link",
            credential["link"],
            "--force",
            check=True,
        )

        xp2p_client_runner(
            "client",
            "redirect",
            "add",
            "--cidr",
            CLIENT_REDIRECT_CIDR,
            "--host",
            server_public_host,
            check=True,
        )
        xp2p_client_runner(
            "client",
            "redirect",
            "add",
            "--cidr",
            CLIENT_REDIRECT_CIDR,
            "--host",
            server_public_host,
            check=True,
        )

        with xp2p_server_run_factory(
            str(INSTALL_DIR),
            SERVER_CONFIG_DIR,
            SERVER_LOG_RELATIVE,
        ), xp2p_client_run_factory(
            str(INSTALL_DIR),
            CLIENT_CONFIG_DIR,
            CLIENT_LOG_RELATIVE,
        ):
            tun_index = _wait_for_interface_index(client_host, CLIENT_TUN)
            _wait_for_route_present(client_host, CLIENT_REDIRECT_CIDR, tun_index)

        xp2p_client_runner(
            "client",
            "redirect",
            "remove",
            "--cidr",
            CLIENT_REDIRECT_CIDR,
            "--host",
            server_public_host,
            check=True,
        )

        with xp2p_server_run_factory(
            str(INSTALL_DIR),
            SERVER_CONFIG_DIR,
            SERVER_LOG_RELATIVE,
        ), xp2p_client_run_factory(
            str(INSTALL_DIR),
            CLIENT_CONFIG_DIR,
            CLIENT_LOG_RELATIVE,
        ):
            _wait_for_route_absent(client_host, CLIENT_REDIRECT_CIDR)
    finally:
        xp2p_client_runner(
            "client",
            "redirect",
            "remove",
            "--cidr",
            CLIENT_REDIRECT_CIDR,
            "--host",
            server_public_host,
            check=False,
        )
        _cleanup_client_install(client_host, xp2p_client_runner)
        _cleanup_server_install(server_host, xp2p_server_runner)


@pytest.mark.host
@pytest.mark.win
def test_windows_server_redirect_routes_os(
    server_host,
    client_host,
    xp2p_server_runner,
    xp2p_client_runner,
    xp2p_server_run_factory,
    xp2p_client_run_factory,
):
    _cleanup_server_install(server_host, xp2p_server_runner)
    _cleanup_client_install(client_host, xp2p_client_runner)
    server_public_host = _server_public_host()
    reverse_tag: str | None = None
    try:
        server_install = xp2p_server_runner(
            "server",
            "install",
            "--host",
            server_public_host,
            "--force",
            check=True,
        )
        credential = _extract_generated_credential(server_install.stdout or "")
        assert credential["link"], "Expected trojan link in server install output"
        reverse_tag = _expected_reverse_tag(credential["user"] or "", server_public_host)

        xp2p_client_runner(
            "client",
            "install",
            "--link",
            credential["link"],
            "--force",
            check=True,
        )

        xp2p_server_runner(
            "server",
            "redirect",
            "add",
            "--cidr",
            SERVER_REDIRECT_CIDR,
            "--tag",
            reverse_tag,
            check=True,
        )

        with xp2p_server_run_factory(
            str(INSTALL_DIR),
            SERVER_CONFIG_DIR,
            SERVER_LOG_RELATIVE,
        ), xp2p_client_run_factory(
            str(INSTALL_DIR),
            CLIENT_CONFIG_DIR,
            CLIENT_LOG_RELATIVE,
        ):
            tun_index = _wait_for_interface_index(server_host, SERVER_TUN)
            _wait_for_route_present(server_host, SERVER_REDIRECT_CIDR, tun_index)

        xp2p_server_runner(
            "server",
            "redirect",
            "remove",
            "--cidr",
            SERVER_REDIRECT_CIDR,
            "--tag",
            reverse_tag,
            check=True,
        )

        with xp2p_server_run_factory(
            str(INSTALL_DIR),
            SERVER_CONFIG_DIR,
            SERVER_LOG_RELATIVE,
        ), xp2p_client_run_factory(
            str(INSTALL_DIR),
            CLIENT_CONFIG_DIR,
            CLIENT_LOG_RELATIVE,
        ):
            _wait_for_route_absent(server_host, SERVER_REDIRECT_CIDR)
    finally:
        if reverse_tag:
            xp2p_server_runner(
                "server",
                "redirect",
                "remove",
                "--cidr",
                SERVER_REDIRECT_CIDR,
                "--tag",
                reverse_tag,
                check=False,
            )
        _cleanup_client_install(client_host, xp2p_client_runner)
        _cleanup_server_install(server_host, xp2p_server_runner)


@pytest.mark.host
@pytest.mark.win
def test_windows_client_redirect_route_switch_and_proxy_cleanup(
    server_host,
    client_host,
    xp2p_server_runner,
    xp2p_client_runner,
    xp2p_server_run_factory,
    xp2p_client_run_factory,
):
    _cleanup_server_install(server_host, xp2p_server_runner)
    _cleanup_client_install(client_host, xp2p_client_runner)
    server_public_host = _server_public_host()
    original_mode: str | None = None
    try:
        server_install = xp2p_server_runner(
            "server",
            "install",
            "--host",
            server_public_host,
            "--force",
            check=True,
        )
        credential = _extract_generated_credential(server_install.stdout or "")
        assert credential["link"], "Expected trojan link in server install output"

        xp2p_client_runner(
            "client",
            "install",
            "--link",
            credential["link"],
            "--force",
            check=True,
        )

        xp2p_client_runner(
            "client",
            "redirect",
            "add",
            "--cidr",
            CLIENT_REDIRECT_CIDR,
            "--host",
            server_public_host,
            check=True,
        )

        with xp2p_server_run_factory(
            str(INSTALL_DIR),
            SERVER_CONFIG_DIR,
            SERVER_LOG_RELATIVE,
        ), xp2p_client_run_factory(
            str(INSTALL_DIR),
            CLIENT_CONFIG_DIR,
            CLIENT_LOG_RELATIVE,
        ):
            tun_index = _wait_for_interface_index(client_host, CLIENT_TUN)
            _wait_for_route_present(client_host, CLIENT_REDIRECT_CIDR, tun_index)

        xp2p_client_runner(
            "client",
            "redirect",
            "remove",
            "--cidr",
            CLIENT_REDIRECT_CIDR,
            "--host",
            server_public_host,
            check=True,
        )
        xp2p_client_runner(
            "client",
            "redirect",
            "add",
            "--cidr",
            CLIENT_REDIRECT_CIDR_ALT,
            "--host",
            server_public_host,
            check=True,
        )

        with xp2p_server_run_factory(
            str(INSTALL_DIR),
            SERVER_CONFIG_DIR,
            SERVER_LOG_RELATIVE,
        ), xp2p_client_run_factory(
            str(INSTALL_DIR),
            CLIENT_CONFIG_DIR,
            CLIENT_LOG_RELATIVE,
        ):
            tun_index = _wait_for_interface_index(client_host, CLIENT_TUN)
            _wait_for_route_absent(client_host, CLIENT_REDIRECT_CIDR)
            _wait_for_route_present(client_host, CLIENT_REDIRECT_CIDR_ALT, tun_index)

        original_mode = _current_client_mode(client_host)
        _set_client_mode(xp2p_client_runner, "proxy")

        with xp2p_server_run_factory(
            str(INSTALL_DIR),
            SERVER_CONFIG_DIR,
            SERVER_LOG_RELATIVE,
        ), xp2p_client_run_factory(
            str(INSTALL_DIR),
            CLIENT_CONFIG_DIR,
            CLIENT_LOG_RELATIVE,
        ):
            _wait_for_route_absent(client_host, CLIENT_REDIRECT_CIDR_ALT)
    finally:
        xp2p_client_runner(
            "client",
            "redirect",
            "remove",
            "--cidr",
            CLIENT_REDIRECT_CIDR,
            "--host",
            server_public_host,
            check=False,
        )
        xp2p_client_runner(
            "client",
            "redirect",
            "remove",
            "--cidr",
            CLIENT_REDIRECT_CIDR_ALT,
            "--host",
            server_public_host,
            check=False,
        )
        if original_mode:
            _set_client_mode(xp2p_client_runner, original_mode)
        _cleanup_client_install(client_host, xp2p_client_runner)
        _cleanup_server_install(server_host, xp2p_server_runner)
