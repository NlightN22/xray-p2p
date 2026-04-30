import time

from testinfra.backend.base import CommandResult
from testinfra.host import Host


def get_host_ipv4(host: Host) -> str:
    from . import env as _env

    script = """
$ErrorActionPreference = 'Stop'
$addresses = Get-NetIPAddress -AddressFamily IPv4 -PrefixOrigin (@('Dhcp', 'Manual')) |
    Where-Object { $_.IPAddress -ne '127.0.0.1' } |
    Select-Object -ExpandProperty IPAddress
if (-not $addresses) {
    exit 3
}
$addresses
"""
    result = _env.run_powershell(host, script, label="read_text")
    if result.rc != 0:
        raise RuntimeError(
            "Failed to detect IPv4 addresses.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    addresses = [line.strip() for line in (result.stdout or "").splitlines() if line.strip()]
    if not addresses:
        raise RuntimeError("No IPv4 addresses found on host")
    for addr in addresses:
        if not addr.startswith("10.0.2."):
            return addr
    return addresses[0]


def get_default_ipv4_sendthrough(host: Host) -> str | None:
    from . import env as _env

    def _read() -> CommandResult:
        return _env.run_guest_script(
            host,
            "scripts/get_default_ipv4_sendthrough.ps1",
        )

    result = _read()
    if result.rc != 0:
        ensure_default_ipv4_route(host, timeout=60.0)
        result = _read()
    if result.rc != 0:
        raise RuntimeError(
            "Failed to detect default IPv4 route address.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    values = [line.strip() for line in (result.stdout or "").splitlines() if line.strip()]
    if not values:
        return None
    return values[-1]


def ensure_default_ipv4_route(host: Host, *, timeout: float = 60.0) -> None:
    from . import env as _env

    script = r"""
$ErrorActionPreference = 'SilentlyContinue'

function Has-DefaultRoute {
    $routes = Get-NetRoute -DestinationPrefix '0.0.0.0/0' -AddressFamily IPv4 -ErrorAction SilentlyContinue |
        Where-Object { $_.NextHop -and $_.NextHop -ne '0.0.0.0' }
    return ($null -ne $routes -and @($routes).Count -gt 0)
}

if (Has-DefaultRoute) {
    Write-Output "OK: default route present"
    exit 0
}

Write-Output "WARN: default route missing; attempting DHCP renew"
try { ipconfig /renew | Out-Null } catch {}
Start-Sleep -Seconds 2
if (Has-DefaultRoute) {
    Write-Output "OK: default route restored after renew"
    exit 0
}

Write-Output "WARN: default route still missing; attempting to recycle connected adapter"
$iface = Get-NetIPInterface -AddressFamily IPv4 -ErrorAction SilentlyContinue |
    Where-Object {
        $_.ConnectionState -eq 'Connected' -and
        $_.InterfaceAlias -notmatch '^xp2p' -and
        $_.InterfaceAlias -notmatch '^Loopback'
    } |
    Sort-Object -Property InterfaceMetric |
    Select-Object -First 1
if ($iface) {
    $alias = $iface.InterfaceAlias
    Disable-NetAdapter -Name $alias -Confirm:$false | Out-Null
    Start-Sleep -Seconds 3
    Enable-NetAdapter -Name $alias -Confirm:$false | Out-Null
    try { ipconfig /renew | Out-Null } catch {}
    Start-Sleep -Seconds 2
    if (Has-DefaultRoute) {
        Write-Output ("OK: default route restored after recycle ({0})" -f $alias)
        exit 0
    }
}

Write-Output "WARN: attempting to add default route from IPv4DefaultGateway"
$cfg = Get-NetIPConfiguration -ErrorAction SilentlyContinue |
    Where-Object { $_.IPv4Address -and $_.NetAdapter -and $_.NetAdapter.Status -eq 'Up' }
$picked = $cfg | Where-Object { $_.IPv4DefaultGateway -and $_.IPv4DefaultGateway.NextHop } | Select-Object -First 1
$ifIndex = $null
$gw = $null
if ($picked) {
    $ifIndex = $picked.InterfaceIndex
    $gw = $picked.IPv4DefaultGateway.NextHop
}
if (-not $gw) {
    $nat = $cfg | Where-Object { $_.IPv4Address.IPAddress -like '10.0.2.*' } | Select-Object -First 1
    if ($nat) {
        $ifIndex = $nat.InterfaceIndex
        $gw = '10.0.2.2'
    }
}
if ($ifIndex -and $gw) {
    try {
        New-NetRoute -DestinationPrefix '0.0.0.0/0' -InterfaceIndex $ifIndex -NextHop $gw -PolicyStore ActiveStore | Out-Null
    } catch {}
    Start-Sleep -Seconds 1
    if (Has-DefaultRoute) {
        Write-Output ("OK: default route added ifIndex={0} gw={1}" -f $ifIndex, $gw)
        exit 0
    }
}

Write-Output "ERROR: failed to restore default route"
Write-Output "=== route print ==="
route print
exit 3
"""
    deadline = time.time() + timeout
    last: CommandResult | None = None
    while time.time() < deadline:
        last = _env.run_powershell(host, script, timeout=60, label="ensure_default_ipv4_route")
        if last.rc == 0:
            return
        time.sleep(2)
    raise RuntimeError(
        "Failed to restore default IPv4 route within timeout.\n"
        f"STDOUT:\n{(last.stdout if last else '')}\nSTDERR:\n{(last.stderr if last else '')}"
    )

