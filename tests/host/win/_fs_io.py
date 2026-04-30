import base64
import json
import time
import uuid
from pathlib import Path

from testinfra.host import Host

try:
    import tomllib
except ImportError:  # pragma: no cover - fallback for older runtimes.
    import tomli as tomllib

from ._fs_path import _as_path, _pending_candidate, _resolve_config_path


def read_text(host: Host, path: Path | str) -> str:
    from . import env as _env

    resolved = _resolve_config_path(host, _as_path(path))
    target = _env.ps_quote(str(resolved))
    script = f"""
$ErrorActionPreference = 'Stop'
$target = {target}
if (-not (Test-Path $target)) {{
    exit 3
}}
Get-Content -Path $target -Raw
exit 0
"""
    result = _env.run_powershell(host, script, label="get_host_ipv4")
    if result.rc != 0:
        raise RuntimeError(
            f"Failed to read remote text {path}.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return result.stdout or ""


def read_toml(host: Host, path: Path | str) -> dict:
    content = read_text(host, path)
    try:
        return tomllib.loads(content)
    except tomllib.TOMLDecodeError as exc:
        raise RuntimeError(f"Failed to parse TOML from {path}: {exc}\nContent:\n{content}") from exc


def write_text(host: Host, path: Path | str, content: str) -> None:
    from . import env as _env

    resolved = _pending_candidate(_as_path(path))
    encoded = base64.b64encode(content.encode("utf-8")).decode("ascii")
    target = _env.ps_quote(str(resolved))
    payload = _env.ps_quote(encoded)
    script = f"""
$ErrorActionPreference = 'Stop'
$target = {target}
$payload = {payload}
$bytes = [System.Convert]::FromBase64String($payload)
$text = [System.Text.Encoding]::UTF8.GetString($bytes)
$dir = Split-Path -Parent $target
if ($dir -and -not (Test-Path $dir)) {{
    New-Item -ItemType Directory -Path $dir -Force | Out-Null
}}
$encoding = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText($target, $text, $encoding)
exit 0
"""
    result = _env.run_powershell(host, script)
    if result.rc != 0:
        raise RuntimeError(
            f"Failed to write remote text {path}.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def write_text_exact(host: Host, path: Path | str, content: str) -> None:
    from . import env as _env

    encoded = base64.b64encode(content.encode("utf-8")).decode("ascii")
    target = _env.ps_quote(str(_as_path(path)))
    payload = _env.ps_quote(encoded)
    script = f"""
$ErrorActionPreference = 'Stop'
$target = {target}
$payload = {payload}
$bytes = [System.Convert]::FromBase64String($payload)
$text = [System.Text.Encoding]::UTF8.GetString($bytes)
$dir = Split-Path -Parent $target
if ($dir -and -not (Test-Path $dir)) {{
    New-Item -ItemType Directory -Path $dir -Force | Out-Null
}}
$encoding = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText($target, $text, $encoding)
exit 0
"""
    result = _env.run_powershell(host, script, label="write_text_exact")
    if result.rc != 0:
        raise RuntimeError(
            f"Failed to write remote text {path}.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def write_apply_request(host: Host, role: str) -> None:
    from . import env as _env

    timestamp = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    payload = json.dumps(
        {
            "id": str(uuid.uuid4()),
            "timestamp": timestamp,
            "role": role,
        }
    )
    payload = f"{payload}\n"
    path = _env.CONFIG_ROOT / _env.APPLY_DIR_NAME / "apply.request"
    write_text(host, path, payload)

