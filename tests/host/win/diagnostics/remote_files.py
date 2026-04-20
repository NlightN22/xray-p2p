from __future__ import annotations

import json
from pathlib import Path

import pytest
from testinfra.host import Host

from tests.host.win import env as win_env


def read_optional_text(host: Host, path_value: Path | str) -> str:
    path = Path(path_value)
    script = f"""
$ErrorActionPreference = 'Stop'
$path = {win_env.ps_quote(str(path))}
if (-not (Test-Path $path)) {{
    exit 3
}}
Get-Content -Path $path -Raw
exit 0
"""
    result = win_env.run_powershell(host, script, label="read_optional_text")
    if result.rc == 0:
        return result.stdout or ""
    if result.rc == 3:
        return ""
    pytest.fail(
        f"Failed to read remote text {path}.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
    )


def read_remote_json(host: Host, path: Path) -> dict:
    resolved = win_env.resolve_config_path(host, path)
    script = f"""
$ErrorActionPreference = 'Stop'
$path = {win_env.ps_quote(str(resolved))}
if (-not (Test-Path $path)) {{
    exit 3
}}
Get-Content -Path $path -Raw
exit 0
"""
    result = win_env.run_powershell(host, script, label="read_remote_json")
    if result.rc != 0:
        pytest.fail(
            f"Failed to read remote json {path}.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    payload = (result.stdout or "").strip()
    if not payload:
        return {}
    try:
        data = json.loads(payload)
    except json.JSONDecodeError as exc:
        pytest.fail(f"Unexpected json payload from {path}: {payload!r} ({exc})")
    if data is None:
        return {}
    if isinstance(data, dict):
        return data
    pytest.fail(f"Unexpected json type from {path}: {type(data).__name__}")


def read_first_existing_json(host: Host, paths: list[Path]) -> dict:
    for path in paths:
        if win_env.path_exists(host, path):
            return read_remote_json(host, path)
    pytest.fail(f"None of the JSON paths exist: {[str(path) for path in paths]}")

