from __future__ import annotations

from pathlib import Path

import json
import pytest

from tests.host.win import env as _env

SERVER_PUBLIC_HOST = "10.0.10.10"
SERVER_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
SERVER_CONFIG_DIR = "config-server"
CLIENT_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
CLIENT_CONFIG_DIR = "config-client"
SERVER_LOG_RELATIVE = r"logs\server.err"
CLIENT_LOG_RELATIVE = r"logs\client.err"
CLIENT_ROUTING_JSON = CLIENT_INSTALL_DIR / CLIENT_CONFIG_DIR / "routing.json"
DIAG_IP = "10.77.0.1"
DIAG_CIDR = f"{DIAG_IP}/32"
DIAG_PREFIX = 32


def _cleanup_server_install(server_host, runner, msi_path: str) -> None:
    runner("server", "remove", "--ignore-missing")
    _env.install_xp2p_from_msi(server_host, msi_path)


def _cleanup_client_install(client_host, runner, msi_path: str) -> None:
    runner("client", "remove", "--all", "--ignore-missing")
    _env.install_xp2p_from_msi(client_host, msi_path)


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


def _ps_exec(host, script: str):
    result = _env.run_powershell(host, script)
    if result.rc != 0:
        pytest.fail(
            "Remote PowerShell command failed.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return result


def _get_interface_alias(host, ip: str) -> str:
    script = f"""
$ErrorActionPreference = 'Stop'
$entry = Get-NetIPAddress -IPAddress {_env.ps_quote(ip)} -AddressFamily IPv4 | Select-Object -First 1
if (-not $entry) {{
    throw "Interface for IP {ip} not found"
}}
Write-Output $entry.InterfaceAlias
"""
    result = _ps_exec(host, script)
    alias = (result.stdout or "").strip()
    if not alias:
        pytest.fail(f"Failed to determine interface alias for {ip}")
    return alias


def _add_ip_alias(host, alias: str, ip: str, prefix: int) -> None:
    script = f"""
$ErrorActionPreference = 'Stop'
Get-NetIPAddress -IPAddress {_env.ps_quote(ip)} -ErrorAction SilentlyContinue | Remove-NetIPAddress -Confirm:$false -ErrorAction SilentlyContinue
New-NetIPAddress -IPAddress {_env.ps_quote(ip)} -PrefixLength {prefix} -InterfaceAlias {_env.ps_quote(alias)} -AddressFamily IPv4 -Type Unicast | Out-Null
"""
    _ps_exec(host, script)


def _remove_ip_alias(host, ip: str) -> None:
    script = f"""
Get-NetIPAddress -IPAddress {_env.ps_quote(ip)} -ErrorAction SilentlyContinue | Remove-NetIPAddress -Confirm:$false -ErrorAction SilentlyContinue
"""
    _env.run_powershell(host, script)


def _read_remote_text(host, path: Path) -> str:
    quoted = _env.ps_quote(str(path))
    script = f"""
$ErrorActionPreference = 'Stop'
$target = {quoted}
if (-not (Test-Path $target)) {{
    return ""
}}
Get-Content -Raw $target
"""
    result = _env.run_powershell(host, script)
    if result.rc != 0:
        pytest.fail(
            f"Failed to read remote text {path}.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return result.stdout or ""


def _read_remote_json(host, path: Path) -> dict:
    content = _read_remote_text(host, path)
    if not content.strip():
        return {}
    try:
        return json.loads(content)
    except json.JSONDecodeError as exc:
        pytest.fail(f"Failed to parse JSON from {path}: {exc}\nContent:\n{content}")


def _expected_tag(host: str) -> str:
    cleaned = host.strip().lower()
    result = []
    last_dash = False
    for char in cleaned:
        if char.isalnum():
            result.append(char)
            last_dash = False
            continue
        if char == "-":
            result.append(char)
            last_dash = False
            continue
        if not last_dash:
            result.append("-")
            last_dash = True
    sanitized = "".join(result).strip("-")
    if not sanitized:
        sanitized = "endpoint"
    return f"proxy-{sanitized}"


def _assert_redirect_rule(data: dict, cidr: str, tag: str) -> None:
    rules = data.get("routing", {}).get("rules", [])
    for rule in rules:
        if rule.get("outboundTag") != tag:
            continue
        ips = rule.get("ip") or []
        if isinstance(ips, list) and len(ips) == 1 and ips[0] == cidr:
            return
    pytest.fail(f"Redirect rule for {cidr} via {tag} not found")


def _assert_no_redirect_rule(data: dict, cidr: str) -> None:
    rules = data.get("routing", {}).get("rules", [])
    for rule in rules:
        ips = rule.get("ip") or []
        if isinstance(ips, list) and cidr in ips:
            pytest.fail(f"Unexpected redirect rule for {cidr}")


@pytest.mark.host
@pytest.mark.win
def test_client_redirect_tunnel_win(
    server_host,
    client_host,
    xp2p_server_runner,
    xp2p_client_runner,
    xp2p_server_run_factory,
    xp2p_client_run_factory,
    xp2p_msi_path,
):
    _cleanup_server_install(server_host, xp2p_server_runner, xp2p_msi_path)
    _cleanup_client_install(client_host, xp2p_client_runner, xp2p_msi_path)
    server_log_path = SERVER_INSTALL_DIR / SERVER_LOG_RELATIVE
    iface = _get_interface_alias(server_host, SERVER_PUBLIC_HOST)
    _remove_ip_alias(server_host, DIAG_IP)
    try:
        _add_ip_alias(server_host, iface, DIAG_IP, DIAG_PREFIX)

        server_install = xp2p_server_runner(
            "--server-host",
            SERVER_PUBLIC_HOST,
            "server",
            "install",
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

        with xp2p_server_run_factory(
            str(SERVER_INSTALL_DIR),
            SERVER_CONFIG_DIR,
            SERVER_LOG_RELATIVE,
        ):
            with xp2p_client_run_factory(
                str(CLIENT_INSTALL_DIR),
                CLIENT_CONFIG_DIR,
                CLIENT_LOG_RELATIVE,
            ):
                initial_ping = xp2p_client_runner(
                    "ping",
                    DIAG_IP,
                    "--socks",
                    "--count",
                    "3",
                    check=False,
                )
                assert initial_ping.rc != 0

                xp2p_client_runner(
                    "client",
                    "redirect",
                    "add",
                    "--cidr",
                    DIAG_CIDR,
                    "--host",
                    SERVER_PUBLIC_HOST,
                    check=True,
                )

                redirected_ping = xp2p_client_runner(
                    "ping",
                    DIAG_IP,
                    "--socks",
                    "--count",
                    "3",
                    check=True,
                )
                redirected_output = (redirected_ping.stdout or "").lower()
                assert "0% loss" in redirected_output

                redirect_list = xp2p_client_runner(
                    "client",
                    "redirect",
                    "list",
                    check=True,
                ).stdout or ""
                assert DIAG_CIDR in redirect_list

                routing = _read_remote_json(client_host, CLIENT_ROUTING_JSON)
                _assert_redirect_rule(routing, DIAG_CIDR, _expected_tag(SERVER_PUBLIC_HOST))

                server_log = _read_remote_text(server_host, server_log_path)
                assert "ping received" in server_log.lower()

                xp2p_client_runner(
                    "client",
                    "redirect",
                    "remove",
                    "--cidr",
                    DIAG_CIDR,
                    check=True,
                )

                routing_after = _read_remote_json(client_host, CLIENT_ROUTING_JSON)
                _assert_no_redirect_rule(routing_after, DIAG_CIDR)

                final_ping = xp2p_client_runner(
                    "ping",
                    DIAG_IP,
                    "--socks",
                    "--count",
                    "3",
                    check=False,
                )
                assert final_ping.rc != 0

                final_list = xp2p_client_runner(
                    "client",
                    "redirect",
                    "list",
                    check=True,
                ).stdout or ""
                assert "no redirect rules configured" in final_list.lower()
    finally:
        _remove_ip_alias(server_host, DIAG_IP)
        _cleanup_client_install(client_host, xp2p_client_runner, xp2p_msi_path)
        _cleanup_server_install(server_host, xp2p_server_runner, xp2p_msi_path)
