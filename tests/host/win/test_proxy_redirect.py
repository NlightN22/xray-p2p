from __future__ import annotations

from pathlib import Path

import json
import pytest

from tests.host import cli_json

from tests.host.win import env as _env
from tests.host.win.assertions import socks as socks_assert
from tests.host.win.diagnostics import net_state as net_diag
SERVER_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
SERVER_CONFIG_DIR = "config-server"
CLIENT_INSTALL_DIR = Path(r"C:\Program Files\xp2p")
CLIENT_CONFIG_DIR = "config-client"
CLIENT_LIVE_XRAY_JSON = _env.CONFIG_LIVE_ROOT / CLIENT_CONFIG_DIR / "xray.json"
DIAG_IP = "10.77.0.1"
DIAG_CIDR = f"{DIAG_IP}/32"
DIAG_PREFIX = 32
DIAG_DOMAIN_IP = "10.77.0.2"
DIAG_DOMAIN = "diag.service.internal"
SERVER_STATE_FILES = [
    _env.CONFIG_ROOT / "xp2p-server.toml",
    _env.CONFIG_ROOT / "xp2p-server.state.json",
]
CLIENT_STATE_FILES = [
    _env.CONFIG_ROOT / "xp2p-client.toml",
    _env.CONFIG_ROOT / "xp2p-client.state.json",
]
CLIENT_NETSTATE_LOG = r"C:\xp2p\build\logs\win\netstate-client.log"
SERVER_NETSTATE_LOG = r"C:\xp2p\build\logs\win\netstate-server.log"


def _server_public_host() -> str:
    return _env.DEFAULT_TARGET


def _cleanup_server_install(server_host, runner, msi_path: str) -> None:
    _stop_xp2p_services(server_host)
    runner("server", "remove", "--ignore-missing")
    _env.cleanup_xp2p_install(
        server_host,
        config_dirs=[_env.CONFIG_ROOT / SERVER_CONFIG_DIR],
        state_files=SERVER_STATE_FILES,
    )


def _cleanup_client_install(client_host, runner, msi_path: str) -> None:
    _stop_xp2p_services(client_host)
    runner("client", "remove", "--all", "--ignore-missing")
    _env.cleanup_xp2p_install(
        client_host,
        config_dirs=[_env.CONFIG_ROOT / CLIENT_CONFIG_DIR],
        state_files=CLIENT_STATE_FILES,
    )


def _parse_json_credential(stdout: str) -> dict[str, str | None]:
    return cli_json.credential(stdout)

def _ps_exec(host, script: str):
    result = _env.run_powershell(host, script)
    if result.rc != 0:
        pytest.fail(
            "Remote PowerShell command failed.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return result


def _stop_xp2p_services(host) -> None:
    script = """
$ErrorActionPreference = 'SilentlyContinue'
foreach ($name in @('xp2p-client', 'xp2p-server')) {
    $svc = Get-Service -Name $name -ErrorAction SilentlyContinue
    if ($svc -and $svc.Status -ne 'Stopped') {
        Stop-Service -Name $name -Force -ErrorAction SilentlyContinue
    }
}
exit 0
"""
    _env.run_powershell(host, script)


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
    resolved = _env.resolve_config_path(host, path)
    quoted = _env.ps_quote(str(resolved))
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


def _assert_domain_redirect_rule(data: dict, domain: str, tag: str) -> None:
    normalized = domain.strip().lower()
    accepted = {normalized, f"domain:{normalized}"}
    rules = data.get("routing", {}).get("rules", [])
    for rule in rules:
        if rule.get("outboundTag") != tag:
            continue
        domains = rule.get("domains") or []
        lowered = [entry.strip().lower() for entry in domains if isinstance(entry, str)]
        if any(entry in accepted for entry in lowered):
            return
    pytest.fail(f"Domain redirect rule for {domain} via {tag} not found")


def _assert_no_domain_redirect_rule(data: dict, domain: str) -> None:
    normalized = domain.strip().lower()
    accepted = {normalized, f"domain:{normalized}"}
    rules = data.get("routing", {}).get("rules", [])
    for rule in rules:
        domains = rule.get("domains") or []
        lowered = [entry.strip().lower() for entry in domains if isinstance(entry, str)]
        if any(entry in accepted for entry in lowered):
            pytest.fail(f"Unexpected domain redirect rule for {domain}")


def _set_firewall_block(host, name: str, addresses: list[str], present: bool = True) -> None:
    quoted_addresses = ", ".join([_env.ps_quote(addr) for addr in addresses])
    ensure = "$true" if present else "$false"
    script = f"""
$ErrorActionPreference = 'Stop'
$ruleName = {_env.ps_quote(name)}
$remote = @({quoted_addresses})
Get-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue | Remove-NetFirewallRule -ErrorAction SilentlyContinue
if ({ensure}) {{
    New-NetFirewallRule -DisplayName $ruleName -Direction Outbound -Action Block -RemoteAddress $remote -Profile Any -Protocol Any | Out-Null
}}
"""
    _ps_exec(host, script)


def _add_hosts_entry(host, ip: str, hostname: str) -> None:
    result = _env.run_guest_script(
        host,
        "scripts/update_hosts_entry.ps1",
        Action="Add",
        HostName=hostname,
        IPAddress=ip,
    )
    if result.rc != 0:
        pytest.fail(
            "Failed to add hosts entry.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def _remove_hosts_entry(host, hostname: str) -> None:
    _env.run_guest_script(
        host,
        "scripts/update_hosts_entry.ps1",
        Action="Remove",
        HostName=hostname,
    )


def _dump_net_state(host, output_path: str, label: str) -> None:
    net_diag.dump_net_state(host, output_path=output_path, label=label)


@pytest.mark.host
@pytest.mark.win
def test_client_redirect_proxy_win(
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
    server_public_host = _server_public_host()
    server_log_path = _env.LOGS_DIR / "xp2p-server-run.out"
    iface = _get_interface_alias(server_host, server_public_host)
    _remove_ip_alias(server_host, DIAG_IP)
    _remove_ip_alias(server_host, DIAG_DOMAIN_IP)
    client_host_entry_added = False
    try:
        _add_hosts_entry(server_host, DIAG_DOMAIN_IP, DIAG_DOMAIN)
        _add_hosts_entry(client_host, DIAG_DOMAIN_IP, DIAG_DOMAIN)
        client_host_entry_added = True
        _dump_net_state(server_host, SERVER_NETSTATE_LOG, "before-server-install")
        _dump_net_state(client_host, CLIENT_NETSTATE_LOG, "before-client-install")

        server_install = xp2p_server_runner(
            "server",
            "install", "--json",
            "--host",
            server_public_host,
            "--force",
            check=True,
            )
        credential = _parse_json_credential(server_install.stdout or "")
        assert credential["link"], "Expected connection link in server install output"

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
            "mode",
            "proxy",
            "--path",
            str(SERVER_INSTALL_DIR),
            "--config-dir",
            SERVER_CONFIG_DIR,
            check=True,
        )
        xp2p_client_runner(
            "client",
            "mode",
            "proxy",
            "--path",
            str(CLIENT_INSTALL_DIR),
            "--config-dir",
            CLIENT_CONFIG_DIR,
            check=True,
        )

        with xp2p_server_run_factory(
            str(SERVER_INSTALL_DIR),
            SERVER_CONFIG_DIR,
            ):
            _dump_net_state(server_host, SERVER_NETSTATE_LOG, "after-server-run-start")
            with xp2p_client_run_factory(
                str(CLIENT_INSTALL_DIR),
                CLIENT_CONFIG_DIR,
                ):
                _dump_net_state(client_host, CLIENT_NETSTATE_LOG, "after-client-run-start")
                socks_assert.wait_for_socks_listener(client_host, port=51180, timeout=30)
                baseline_ping = xp2p_client_runner(
                    "ping",
                    server_public_host,
                    "--tunnel=127.0.0.1:51180",
                    "--count",
                    "3",
                    "--timeout",
                    "5",
                    check=True,
                    )
                assert "0% loss" in (baseline_ping.stdout or "").lower()
                initial_ping = xp2p_client_runner(
                    "ping",
                    DIAG_IP,
                    "--tunnel=127.0.0.1:51180",
                    "--count",
                    "3",
                    "--timeout",
                    "5",
                    check=False,
                    )
                assert initial_ping.rc != 0

                _add_ip_alias(server_host, iface, DIAG_IP, DIAG_PREFIX)
                _add_ip_alias(server_host, iface, DIAG_DOMAIN_IP, DIAG_PREFIX)

                xp2p_client_runner(
                    "client",
                    "redirect",
                    "add",
                    "--cidr",
                    DIAG_CIDR,
                    "--host",
                    server_public_host,
                    check=True,
                    )

            with xp2p_client_run_factory(
                str(CLIENT_INSTALL_DIR),
                CLIENT_CONFIG_DIR,
                ):
                socks_assert.wait_for_socks_listener(client_host, port=51180, timeout=30)
                redirected_ping = xp2p_client_runner(
                    "ping",
                    DIAG_IP,
                    "--tunnel=127.0.0.1:51180",
                    "--count",
                    "3",
                    "--timeout",
                    "5",
                    check=True,
                    )
                assert "0% loss" in (redirected_ping.stdout or "").lower()

                redirect_list = xp2p_client_runner(
                    "client",
                    "redirect",
                    check=True,
                ).stdout or ""
                assert DIAG_CIDR in redirect_list

                routing = _read_remote_json(client_host, CLIENT_LIVE_XRAY_JSON)
                _assert_redirect_rule(routing, DIAG_CIDR, _expected_tag(server_public_host))

                server_log = _read_remote_text(server_host, server_log_path)
                assert server_log.strip(), "Server log is empty"

                xp2p_client_runner(
                    "client",
                    "redirect",
                    "add",
                    "--domain",
                    DIAG_DOMAIN,
                    "--host",
                    server_public_host,
                    check=True,
                    )

                redirect_list = xp2p_client_runner(
                    "client",
                    "redirect",
                    check=True,
                ).stdout or ""
                assert DIAG_DOMAIN in redirect_list

                xp2p_client_runner(
                    "client",
                    "redirect",
                    "remove",
                    "--domain",
                    DIAG_DOMAIN,
                    "--host",
                    server_public_host,
                    check=True,
                    )

                redirect_list = xp2p_client_runner(
                    "client",
                    "redirect",
                    check=True,
                ).stdout or ""
                assert DIAG_DOMAIN not in redirect_list

                redirected_ping_again = xp2p_client_runner(
                    "ping",
                    DIAG_IP,
                    "--tunnel=127.0.0.1:51180",
                    "--count",
                    "3",
                    "--timeout",
                    "5",
                    check=True,
                    )
                assert "0% loss" in (redirected_ping_again.stdout or "").lower()

                xp2p_client_runner(
                    "client",
                    "redirect",
                    "remove",
                    "--cidr",
                    DIAG_CIDR,
                    "--host",
                    server_public_host,
                    check=True,
                    )

                redirect_list = xp2p_client_runner(
                    "client",
                    "redirect",
                    check=True,
                ).stdout or ""
                assert DIAG_CIDR not in redirect_list

            with xp2p_client_run_factory(
                str(CLIENT_INSTALL_DIR),
                CLIENT_CONFIG_DIR,
                ):
                socks_assert.wait_for_socks_listener(client_host, port=51180, timeout=30)
                _remove_ip_alias(server_host, DIAG_IP)
                _remove_ip_alias(server_host, DIAG_DOMAIN_IP)

                final_ping = xp2p_client_runner(
                    "ping",
                    DIAG_IP,
                    "--tunnel=127.0.0.1:51180",
                    "--count",
                    "3",
                    "--timeout",
                    "5",
                    check=False,
                    )
                assert final_ping.rc != 0

                final_list = xp2p_client_runner(
                    "client",
                    "redirect",
                    check=True,
                ).stdout or ""
                assert "no redirect rules configured" in final_list.lower()
    finally:
        _remove_ip_alias(server_host, DIAG_IP)
        _remove_ip_alias(server_host, DIAG_DOMAIN_IP)
        _remove_hosts_entry(server_host, DIAG_DOMAIN)
        if client_host_entry_added:
            _remove_hosts_entry(client_host, DIAG_DOMAIN)
        _dump_net_state(server_host, SERVER_NETSTATE_LOG, "after-cleanup")
        _dump_net_state(client_host, CLIENT_NETSTATE_LOG, "after-cleanup")
        _cleanup_client_install(client_host, xp2p_client_runner, xp2p_msi_path)
        _cleanup_server_install(server_host, xp2p_server_runner, xp2p_msi_path)
