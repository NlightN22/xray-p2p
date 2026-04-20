from __future__ import annotations

import json
import time
from pathlib import Path

import pytest
from testinfra.host import Host

from tests.host.win import env as win_env


def path_exists(host: Host, path_value: Path | str) -> bool:
    path = Path(path_value)
    quoted = win_env.ps_quote(str(path))
    result = win_env.run_powershell(
        host,
        f"if (Test-Path {quoted}) {{ exit 0 }} else {{ exit 3 }}",
        label="live_path_exists",
    )
    return result.rc == 0


def read_text(host: Host, path_value: Path | str, *, label: str) -> str:
    path = Path(path_value)
    quoted = win_env.ps_quote(str(path))
    script = f"""
$ErrorActionPreference = 'Stop'
if (-not (Test-Path {quoted})) {{
    exit 3
}}
Get-Content -Path {quoted} -Raw
exit 0
"""
    result = win_env.run_powershell(host, script, label=label)
    if result.rc != 0:
        pytest.fail(
            f"Failed to read remote file {path}.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return result.stdout or ""


def read_json(host: Host, path_value: Path | str, *, label: str) -> dict:
    payload = read_text(host, path_value, label=label).strip()
    if not payload:
        return {}
    try:
        data = json.loads(payload)
    except json.JSONDecodeError as exc:
        pytest.fail(f"Failed to parse JSON from {path_value}: {exc}\nContent:\n{payload}")
    if data is None:
        return {}
    if isinstance(data, dict):
        return data
    pytest.fail(f"Unexpected JSON payload type from {path_value}: {type(data).__name__}")


def wait_for_exists(
    host: Host,
    path_value: Path | str,
    *,
    timeout: float,
    poll_seconds: float = 1.0,
) -> None:
    deadline = time.time() + timeout
    while time.time() < deadline:
        if path_exists(host, path_value):
            return
        time.sleep(poll_seconds)
    pytest.fail(f"Timed out waiting for file to exist: {path_value}")

