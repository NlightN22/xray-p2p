from __future__ import annotations

import time
import json
from pathlib import Path

import pytest

from tests.host.win import env as _env
from tests.host.win.flows import apply as apply_flow

INSTALL_DIR = Path(r"C:\Program Files\xp2p")
CLIENT_CONFIG_DIR = "config-client"
SERVER_CONFIG_DIR = "config-server"
CLIENT_TUN = "xp2pc"
SERVER_TUN = "xp2ps"

CLIENT_REDIRECT_CIDR = "10.88.0.1/32"
CLIENT_REDIRECT_CIDR_ALT = "10.88.0.2/32"
SERVER_REDIRECT_CIDR = "10.88.1.1/32"
DEFAULT_CLIENT_TUN_ADDR = "198.18.0.1/30"
DEFAULT_SERVER_TUN_ADDR = "198.18.0.5/30"

SERVER_STATE_FILES = [
    _env.CONFIG_ROOT / "xp2p-server.toml",
    _env.CONFIG_ROOT / "xp2p-server.state.json",
]
CLIENT_STATE_FILES = [
    _env.CONFIG_ROOT / "xp2p-client.toml",
    _env.CONFIG_ROOT / "xp2p-client.state.json",
]

ROUTE_WAIT_TIMEOUT = 60.0
ROUTE_POLL_INTERVAL = 1.0


def _server_public_host() -> str:
    return _env.DEFAULT_TARGET


def _cleanup_server_install(server_host, runner) -> None:
    _env.stop_xp2p_processes(server_host)
    runner("server", "remove", "--ignore-missing")
    _env.cleanup_xp2p_install(
        server_host,
        config_dirs=[_env.CONFIG_ROOT / SERVER_CONFIG_DIR],
        state_files=SERVER_STATE_FILES,
    )


def _cleanup_client_install(client_host, runner) -> None:
    _env.stop_xp2p_processes(client_host)
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


def _reverse_tag_from_reverse_list(stdout: str, user: str) -> str:
    if "no reverse tunnels configured" in (stdout or "").strip().lower():
        pytest.fail(f"No reverse tunnels configured.\nSTDOUT:\n{stdout}")

    for raw_line in (stdout or "").splitlines():
        line = raw_line.strip()
        if not line or line.lower().startswith("domain"):
            continue
        parts = line.split()
        if len(parts) < 6:
            continue
        # DOMAIN HOST USER OUTBOUND_TAG PORTAL ROUTING_RULE
        if parts[2] != user:
            continue
        tag = parts[3].strip()
        if tag:
            return tag

    pytest.fail(f"Unable to detect reverse tag for user {user!r}.\nSTDOUT:\n{stdout}")


def _toml_string(value: str) -> str:
    return '"' + value.replace("\\", "\\\\").replace('"', '\\"') + '"'


def _ensure_tun_defaults(host, *, role: str, tun_name: str, tun_addr: str) -> None:
    if role not in {"client", "server"}:
        raise ValueError(f"Unexpected role: {role!r}")
    config_path = _env.CONFIG_ROOT / f"xp2p-{role}.toml"
    content = _env.read_text(host, config_path)
    if not content.strip():
        pytest.fail(f"Expected config at {config_path} to exist after install")

    section_header = f"[{role}]"
    lines = content.splitlines()
    try:
        start = next(i for i, line in enumerate(lines) if line.strip() == section_header)
    except StopIteration:
        pytest.fail(f"Expected {section_header} section in {config_path}.\nContent:\n{content}")

    end = len(lines)
    for i in range(start + 1, len(lines)):
        if lines[i].startswith("[") and lines[i].strip().endswith("]"):
            end = i
            break

    existing_keys: set[str] = set()
    for raw in lines[start + 1 : end]:
        stripped = raw.strip()
        if not stripped or stripped.startswith("#") or stripped.startswith(";"):
            continue
        if "=" not in stripped:
            continue
        key = stripped.split("=", 1)[0].strip()
        if key:
            existing_keys.add(key)

    additions: list[str] = []
    if "tun_enabled" not in existing_keys:
        additions.append("tun_enabled = true")
    if "tun_name" not in existing_keys:
        additions.append(f"tun_name = {_toml_string(tun_name)}")
    if "tun_addr" not in existing_keys:
        additions.append(f"tun_addr = {_toml_string(tun_addr)}")

    if not additions:
        return

    updated = lines[:end] + additions + lines[end:]
    new_content = "\n".join(updated).rstrip("\n") + "\n"
    _env.write_text(host, config_path, new_content)


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
        fallback = _find_interface_index_by_prefix(host, name)
        if fallback is not None:
            return fallback
        time.sleep(ROUTE_POLL_INTERVAL)
    pytest.fail(f"Interface {name} not available: {last_error}")


def _wait_for_tun_ipv4(host, name: str) -> str:
    deadline = time.time() + ROUTE_WAIT_TIMEOUT
    target = _env.ps_quote(name)
    script = f"""
$ErrorActionPreference = 'Stop'
$name = {target}
$ip = Get-NetIPAddress -InterfaceAlias $name -AddressFamily IPv4 -ErrorAction SilentlyContinue |
    Where-Object {{ $_.IPAddress -notlike '169.254.*' -and $_.IPAddress -ne '127.0.0.1' -and $_.IPAddress -ne '0.0.0.0' }} |
    Sort-Object PrefixLength -Descending |
    Select-Object -First 1
if (-not $ip) {{
    exit 3
}}
Write-Output ($ip.IPAddress + '/' + $ip.PrefixLength)
"""
    last_stdout = ""
    last_stderr = ""
    while time.time() < deadline:
        result = _env.run_powershell(host, script, label="wait_tun_ipv4")
        last_stdout = result.stdout or ""
        last_stderr = result.stderr or ""
        if result.rc == 0:
            value = (result.stdout or "").strip().splitlines()
            if value and value[-1].strip():
                return value[-1].strip()
        time.sleep(ROUTE_POLL_INTERVAL)

    dump_path = _env.dump_failure_state(host, label="tun-ip-missing")
    pytest.skip(
        f"TUN interface {name} did not receive an IPv4 address (non-APIPA) within {ROUTE_WAIT_TIMEOUT} seconds.\n"
        f"Last STDOUT:\n{last_stdout}\nLast STDERR:\n{last_stderr}\nFailure dump: {dump_path}"
    )


def _find_interface_index_by_prefix(host, prefix: str) -> int | None:
    target = _env.ps_quote(prefix)
    script = f"""
$ErrorActionPreference = 'Stop'
$prefix = {target}
$adapter = Get-NetAdapter -IncludeHidden -ErrorAction SilentlyContinue |
    Where-Object {{ $_.Name -like "$prefix*" }} |
    Sort-Object -Property ifIndex |
    Select-Object -First 1
if (-not $adapter) {{
    exit 3
}}
Write-Output $adapter.ifIndex
"""
    result = _env.run_powershell(host, script, label="find_net_adapter")
    if result.rc != 0:
        return None
    value = (result.stdout or "").strip().splitlines()
    if not value:
        return None
    try:
        return int(value[-1].strip())
    except ValueError:
        return None


def _route_snapshot(host, cidr: str) -> list[dict]:
    try:
        return _env.get_net_routes(host, cidr)
    except Exception as exc:
        pytest.fail(f"Failed to read routes for {cidr}: {exc}")


def _route_snapshot_by_interface(host, interface_index: int) -> list[dict]:
    script = f"""
$ErrorActionPreference = 'Stop'
$routes = Get-NetRoute -InterfaceIndex {interface_index} -ErrorAction SilentlyContinue |
    Select-Object DestinationPrefix,InterfaceIndex,InterfaceAlias,NextHop,RouteMetric
$routes | ConvertTo-Json -Depth 4 -Compress
"""
    result = _env.run_powershell(host, script, label="get_routes_by_interface")
    if result.rc != 0:
        return []
    payload = (result.stdout or "").strip()
    if not payload:
        return []
    try:
        data = json.loads(payload)
    except Exception:
        return []
    if data is None:
        return []
    if isinstance(data, dict):
        return [data]
    if isinstance(data, list):
        return data
    return []


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
    interface_routes = _route_snapshot_by_interface(host, interface_index)
    pytest.fail(
        f"Timed out waiting for route {cidr}. "
        f"Last exact routes: {last_routes}. "
        f"Routes on interface {interface_index}: {interface_routes}"
    )


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

        original_mode = _current_client_mode(client_host)
        if original_mode != "tun":
            _set_client_mode(xp2p_client_runner, "tun")

        with xp2p_server_run_factory(
            str(INSTALL_DIR),
            SERVER_CONFIG_DIR,
        ), xp2p_client_run_factory(
            str(INSTALL_DIR),
            CLIENT_CONFIG_DIR,
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
        ), xp2p_client_run_factory(
            str(INSTALL_DIR),
            CLIENT_CONFIG_DIR,
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
        if original_mode and original_mode != "tun":
            _set_client_mode(xp2p_client_runner, original_mode)
        _cleanup_client_install(client_host, xp2p_client_runner)
        _cleanup_server_install(server_host, xp2p_server_runner)


@pytest.mark.host
@pytest.mark.win
def test_windows_server_redirect_routes_os(
    server_host,
    client_host,
    xp2p_server_runner,
    xp2p_client_runner,
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
        xp2p_server_runner(
            "server",
            "mode",
            "tun",
            "--path",
            str(INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR,
            check=True,
        )
        xp2p_server_runner(
            "server",
            "user",
            "add",
            "--path",
            str(INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR,
            "--id",
            credential["user"] or "",
            "--password",
            credential["password"] or "",
            "--host",
            server_public_host,
            "--force",
            check=True,
        )
        reverse_list = xp2p_server_runner("server", "reverse", "list", check=True).stdout or ""
        reverse_tag = _reverse_tag_from_reverse_list(reverse_list, credential["user"] or "")

        xp2p_client_runner(
            "client",
            "install",
            "--link",
            credential["link"],
            "--force",
            check=True,
            )
        _ensure_tun_defaults(
            server_host,
            role="server",
            tun_name=SERVER_TUN,
            tun_addr=DEFAULT_SERVER_TUN_ADDR,
        )
        _ensure_tun_defaults(
            client_host,
            role="client",
            tun_name=CLIENT_TUN,
            tun_addr=DEFAULT_CLIENT_TUN_ADDR,
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
        apply_flow.wait_for_apply_request_set(
            server_host,
            timeout=10.0,
            dump_label="server-redirect-add",
        )
        xp2p_server_runner("server", "service", "start", check=True)
        xp2p_client_runner("client", "service", "start", check=True)
        apply_flow.wait_for_apply_request_clear(server_host, timeout=90.0, dump_label="server-redirect-apply")
        apply_flow.wait_for_apply_request_clear(client_host, timeout=90.0, dump_label="client-redirect-apply")
        tun_index = _wait_for_interface_index(server_host, SERVER_TUN)
        _wait_for_tun_ipv4(server_host, SERVER_TUN)
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
        apply_flow.wait_for_apply_request_set(
            server_host,
            timeout=10.0,
            dump_label="server-redirect-remove",
        )
        apply_flow.wait_for_apply_request_clear(server_host, timeout=90.0, dump_label="server-redirect-remove-apply")
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
        xp2p_client_runner("client", "service", "stop", check=False)
        xp2p_server_runner("server", "service", "stop", check=False)
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

        original_mode = _current_client_mode(client_host)
        if original_mode != "tun":
            _set_client_mode(xp2p_client_runner, "tun")

        with xp2p_server_run_factory(
            str(INSTALL_DIR),
            SERVER_CONFIG_DIR,
        ), xp2p_client_run_factory(
            str(INSTALL_DIR),
            CLIENT_CONFIG_DIR,
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
        ), xp2p_client_run_factory(
            str(INSTALL_DIR),
            CLIENT_CONFIG_DIR,
            ):
            tun_index = _wait_for_interface_index(client_host, CLIENT_TUN)
            _wait_for_route_absent(client_host, CLIENT_REDIRECT_CIDR)
            _wait_for_route_present(client_host, CLIENT_REDIRECT_CIDR_ALT, tun_index)

        _set_client_mode(xp2p_client_runner, "proxy")

        with xp2p_server_run_factory(
            str(INSTALL_DIR),
            SERVER_CONFIG_DIR,
        ), xp2p_client_run_factory(
            str(INSTALL_DIR),
            CLIENT_CONFIG_DIR,
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
        if original_mode and original_mode != _current_client_mode(client_host):
            _set_client_mode(xp2p_client_runner, original_mode)
        _cleanup_client_install(client_host, xp2p_client_runner)
        _cleanup_server_install(server_host, xp2p_server_runner)
