import base64
import json
from collections.abc import Iterable

from testinfra.host import Host


def stop_xp2p_processes(host: Host) -> None:
    from . import env as _env

    script = """
$ErrorActionPreference = 'Stop'
Get-Process -Name xp2p,xray -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
exit 0
"""
    result = _env.run_powershell(host, script, label="remove_paths")
    if result.rc != 0:
        raise RuntimeError(
            "Failed to stop xp2p processes.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def service_exists(host: Host, service_name: str) -> bool:
    from . import env as _env

    result = _env.run_guest_script(
        host,
        "scripts/check_service_exists.ps1",
        ServiceName=service_name,
    )
    return result.rc == 0


def remove_services(host: Host, services: Iterable[str]) -> None:
    from . import env as _env

    payload = json.dumps([str(service) for service in services])
    if not payload:
        return
    encoded = base64.b64encode(payload.encode("utf-8")).decode("ascii")
    script = f"""
$ErrorActionPreference = 'Stop'
$payload = {_env.ps_quote(encoded)}
$services = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($payload)) | ConvertFrom-Json
foreach ($name in $services) {{
    if (-not $name) {{
        continue
    }}
    Stop-Service -Name $name -Force -ErrorAction SilentlyContinue
    & sc.exe delete $name | Out-Null
}}
exit 0
"""
    result = _env.run_powershell(host, script, label="stop_xp2p_processes")
    if result.rc != 0:
        raise RuntimeError(
            "Failed to remove services on remote host.\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )

