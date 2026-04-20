from __future__ import annotations

from pathlib import Path

import pytest
from testinfra.host import Host

from tests.host.win import env as win_env
from tests.host.win.diagnostics import live_files
from tests.host.win.flows import apply as apply_flow


def ensure_live_xray_json(
    host: Host,
    runner,
    *,
    role: str,
    xray_path: Path,
    apply_timeout: float = 90.0,
    create_timeout: float = 30.0,
    stop_service: bool = True,
) -> None:
    if live_files.path_exists(host, xray_path):
        return
    service_name = f"xp2p-{role}"
    if not win_env.service_exists(host, service_name):
        pytest.skip(f"{service_name} service is not registered; MSI install required.")
    runner(role, "service", "start", check=True)
    apply_flow.wait_for_apply_request_clear(host, timeout=apply_timeout)
    live_files.wait_for_exists(host, xray_path, timeout=create_timeout, poll_seconds=1.0)
    if stop_service:
        runner(role, "service", "stop", check=True)
