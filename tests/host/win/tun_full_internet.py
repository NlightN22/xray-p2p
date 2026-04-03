from __future__ import annotations

import time

import pytest

from tests.host.win import env as _env

ROUTE_POLL_INTERVAL = 2.0


def check_internet_access(host) -> tuple[bool, str]:
    script = """
$ErrorActionPreference = 'Stop'
$dnsName = "example.com"
$tcpHost = "1.1.1.1"
$tcpPort = 443
try {
    Resolve-DnsName -Name $dnsName -ErrorAction Stop | Out-Null
} catch {
    Write-Error "Internet check failed: DNS lookup for $dnsName"
    exit 1
}
try {
    $tcpOk = Test-NetConnection -ComputerName $tcpHost -Port $tcpPort -InformationLevel Quiet
} catch {
    $tcpOk = $false
}
if (-not $tcpOk) {
    Write-Error "Internet check failed: TCP connect to ${tcpHost}:${tcpPort}"
    exit 1
}
exit 0
"""
    result = _env.run_powershell(host, script, label="check_internet_access")
    if result.rc != 0:
        return False, (
            "Client internet check failed.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return True, ""


def check_dns_resolution(host, name: str) -> tuple[bool, str]:
    quoted = _env.ps_quote(name)
    script = f"""
$ErrorActionPreference = 'Stop'
$dnsName = {quoted}
try {{
    Resolve-DnsName -Name $dnsName -ErrorAction Stop | Out-Null
}} catch {{
    Write-Error "DNS check failed: lookup for $dnsName"
    exit 1
}}
exit 0
"""
    result = _env.run_powershell(host, script, label="check_dns_resolution")
    if result.rc != 0:
        return False, (
            "DNS resolution check failed.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return True, ""


def check_direct_ping(host, target: str) -> tuple[bool, str]:
    quoted = _env.ps_quote(target)
    script = f"""
$ErrorActionPreference = 'Stop'
$target = {quoted}
try {{
    $ok = Test-Connection -ComputerName $target -Count 2 -Quiet
}} catch {{
    $ok = $false
}}
if (-not $ok) {{
    Write-Error "Direct ping failed: $target"
    exit 1
}}
exit 0
"""
    result = _env.run_powershell(host, script, label="check_direct_ping")
    if result.rc != 0:
        return False, (
            "Direct ping check failed.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return True, ""


def _collect_internet_debug(host) -> str:
    script = f"""
$ErrorActionPreference = 'SilentlyContinue'
Write-Output "=== Default routes (IPv4) ==="
Get-NetRoute -DestinationPrefix "0.0.0.0/0" |
    Format-Table ifIndex,InterfaceAlias,NextHop,RouteMetric,ifMetric,PolicyStore -AutoSize | Out-String
Write-Output "=== Interfaces (IPv4) ==="
Get-NetIPInterface -AddressFamily IPv4 |
    Format-Table ifIndex,InterfaceAlias,ConnectionState,InterfaceMetric -AutoSize | Out-String
Write-Output "=== DNS servers ==="
Get-DnsClientServerAddress |
    Format-Table InterfaceAlias,InterfaceIndex,AddressFamily,ServerAddresses -AutoSize | Out-String
Write-Output "=== route print ==="
route print
Write-Output "=== xp2p client tun state ==="
if (Test-Path '{_env.CONFIG_ROOT}\\xp2p-client.tun-full.json') {{
    Get-Content -Path '{_env.CONFIG_ROOT}\\xp2p-client.tun-full.json' -Raw
}} else {{
    Write-Output "missing: {_env.CONFIG_ROOT}\\xp2p-client.tun-full.json"
}}
Write-Output "=== xp2p client config ==="
if (Test-Path '{_env.CONFIG_ROOT}\\xp2p-client.toml') {{
    Get-Content -Path '{_env.CONFIG_ROOT}\\xp2p-client.toml' -Raw
}} else {{
    Write-Output "missing: {_env.CONFIG_ROOT}\\xp2p-client.toml"
}}
Write-Output "=== xp2p client service log (tail) ==="
if (Test-Path '{_env.LOG_ROOT}\\client\\service.log') {{
    Get-Content -Path '{_env.LOG_ROOT}\\client\\service.log' -Tail 200
}} else {{
    Write-Output "missing: {_env.LOG_ROOT}\\client\\service.log"
}}
Write-Output "=== xray client service log (tail) ==="
"""
    result = _env.run_powershell(host, script, label="internet_debug")
    return f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"


def assert_internet_access(host, *, label: str = "") -> None:
    ok, detail = check_internet_access(host)
    if ok:
        return
    debug = _collect_internet_debug(host)
    suffix = f" ({label})" if label else ""
    pytest.fail(f"{detail}\nInternet debug{suffix}:\n{debug}")


def assert_dns_resolution(host, name: str, *, label: str = "") -> None:
    ok, detail = check_dns_resolution(host, name)
    if ok:
        return
    debug = _collect_internet_debug(host)
    suffix = f" ({label})" if label else ""
    pytest.fail(f"{detail}\nDNS debug{suffix}:\n{debug}")


def assert_direct_ping(host, target: str, *, label: str = "") -> None:
    ok, detail = check_direct_ping(host, target)
    if ok:
        return
    debug = _collect_internet_debug(host)
    suffix = f" ({label})" if label else ""
    pytest.fail(f"{detail}\nDirect ping debug{suffix}:\n{debug}")


def ensure_internet_access_with_adapter_reset(host) -> None:
    ok, detail = check_internet_access(host)
    if ok:
        return
    script = """
$ErrorActionPreference = 'SilentlyContinue'
$route = Get-NetRoute -DestinationPrefix '0.0.0.0/0' |
    Where-Object { $_.InterfaceAlias -notmatch '^xp2p' } |
    Sort-Object -Property RouteMetric |
    Select-Object -First 1
if (-not $route) {
    Write-Output "No default route found for non-xp2p interface."
    exit 2
}
$alias = $route.InterfaceAlias
Disable-NetAdapter -Name $alias -Confirm:$false | Out-Null
Start-Sleep -Seconds 3
Enable-NetAdapter -Name $alias -Confirm:$false | Out-Null
Write-Output ("Recycled interface: {0}" -f $alias)
"""
    reset_result = _env.run_powershell(host, script, label="recycle_default_adapter")
    deadline = time.time() + 60.0
    last_detail = detail
    while time.time() < deadline:
        ok, last_detail = check_internet_access(host)
        if ok:
            return
        time.sleep(ROUTE_POLL_INTERVAL)
    debug = _collect_internet_debug(host)
    pytest.fail(
        "Client internet check failed after adapter reset.\n"
        f"Reset output:\n{reset_result.stdout}\n{reset_result.stderr}\n{last_detail}\n{debug}"
    )


def ensure_internet_or_skip(host, label: str) -> None:
    ok, detail = check_internet_access(host)
    if ok:
        return
    try:
        ensure_internet_access_with_adapter_reset(host)
    except pytest.fail.Exception:
        pytest.skip(f"Internet access not available on {label}.\n{detail}")
