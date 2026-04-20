from __future__ import annotations

import time

import pytest
from testinfra.host import Host

from tests.host.win import env as win_env

DEFAULT_POLL_SECONDS = 1.0


def get_service_status(host: Host, name: str) -> str | None:
    service_name = win_env.ps_quote(name)
    script = f"""
$ErrorActionPreference = 'Stop'
$service = Get-Service -Name {service_name} -ErrorAction SilentlyContinue
if (-not $service) {{
    exit 3
}}
Write-Output $service.Status
exit 0
"""
    result = win_env.run_powershell(host, script, label="get_service_status")
    if result.rc != 0:
        return None
    text = (result.stdout or "").strip()
    return text.lower() if text else None


def wait_for_service_status(
    host: Host,
    *,
    name: str,
    expected: str,
    timeout: float,
    poll_seconds: float = DEFAULT_POLL_SECONDS,
    dump_label: str | None = None,
) -> None:
    expected_lower = expected.lower()
    deadline = time.monotonic() + float(timeout)
    last_status = None
    while time.monotonic() < deadline:
        last_status = get_service_status(host, name)
        if last_status == expected_lower:
            return
        time.sleep(poll_seconds)

    dump_path = None
    if dump_label:
        try:
            dump_path = win_env.dump_failure_state(host, label=dump_label)
        except Exception as exc:  # noqa: BLE001
            print(f"WARNING: failed to dump failure state ({dump_label}): {exc}")

    message = f"Windows service {name} did not reach status {expected!r}. Last: {last_status!r}"
    if dump_path:
        message = f"{message}\nFailure dump: {dump_path}"
    pytest.fail(message)

