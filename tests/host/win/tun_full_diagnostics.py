from __future__ import annotations

from pathlib import Path

from tests.host.win import env as _env


def read_log_tail(host, path: Path, lines: int = 200) -> str:
    if not _env.path_exists(host, path):
        return ""
    target = _env.ps_quote(str(path))
    script = f"""
$ErrorActionPreference = 'Stop'
$content = Get-Content -Path {target} -Tail {int(lines)}
$content
"""
    result = _env.run_powershell(host, script, label="log_tail")
    if result.rc != 0:
        return f"<failed to read {path}>"
    return result.stdout or ""


def collect_restore_debug(host, tun_name: str, config_file: Path, state_file: Path, service_log: Path) -> str:
    service_tail = read_log_tail(host, service_log)
    script = f"""
$ErrorActionPreference = 'SilentlyContinue'
$result = @{{}}
$result.ServiceStatus = (Get-Service -Name 'xp2p-client' -ErrorAction SilentlyContinue | Select-Object -Property Status,StartType)
$result.Processes = (Get-Process -Name xp2p,xray -ErrorAction SilentlyContinue | Select-Object -Property Name,Id,StartTime)
$result.NetAdapters = (Get-NetAdapter -IncludeHidden -ErrorAction SilentlyContinue |
    Select-Object -Property Name,InterfaceDescription,Status,ifIndex,MacAddress)
$result.NetIpInterfaces = (Get-NetIPInterface -AddressFamily IPv4 -ErrorAction SilentlyContinue |
    Select-Object -Property InterfaceAlias,InterfaceIndex,ConnectionState,InterfaceMetric)
$result.DefaultRoutes = (Get-NetRoute -DestinationPrefix '0.0.0.0/0' -ErrorAction SilentlyContinue |
    Select-Object -Property DestinationPrefix,InterfaceAlias,InterfaceIndex,NextHop,RouteMetric,PolicyStore)
$result.TunRoutes = (Get-NetRoute -DestinationPrefix '0.0.0.0/0' -ErrorAction SilentlyContinue |
    Where-Object {{ $_.InterfaceAlias -eq '{tun_name}' }} |
    Select-Object -Property DestinationPrefix,InterfaceAlias,InterfaceIndex,NextHop,RouteMetric,PolicyStore)
$result.TunAllRoutes = (Get-NetRoute -ErrorAction SilentlyContinue |
    Where-Object {{ $_.InterfaceAlias -eq '{tun_name}' }} |
    Select-Object -Property DestinationPrefix,InterfaceAlias,InterfaceIndex,NextHop,RouteMetric,PolicyStore)
$result.ClientConfig = (Get-Content -Path { _env.ps_quote(str(config_file)) } -Raw -ErrorAction SilentlyContinue)
$result.TunState = (Get-Content -Path { _env.ps_quote(str(state_file)) } -Raw -ErrorAction SilentlyContinue)
$result.DnsServers = (Get-DnsClientServerAddress -AddressFamily IPv4 -ErrorAction SilentlyContinue |
    Select-Object -Property InterfaceAlias,InterfaceIndex,ServerAddresses)
$result
"""
    result = _env.run_powershell(host, script, label="restore_debug")
    if result.rc != 0:
        return f"<failed to collect restore debug> STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}\nservice_log_tail:\n{service_tail}"
    return f"{result.stdout}\nservice_log_tail:\n{service_tail}"


def fail_with_restore_debug(
    host,
    tun_name: str,
    debug: str,
    *,
    config_file: Path,
    state_file: Path,
    service_log: Path,
) -> None:
    restore_debug = collect_restore_debug(host, tun_name, config_file, state_file, service_log)
    raise AssertionError(f"{debug}\nrestore_debug:\n{restore_debug}")
