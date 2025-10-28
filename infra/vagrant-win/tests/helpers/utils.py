from __future__ import annotations

import time
from typing import Dict, Tuple

import pytest

from .constants import SETUP_URL

_PROVISIONED: Dict[Tuple[str, str, str, str], object] = {}


def run_checked(host, command: str, description: str):
    """Run a shell command via testinfra host and assert a zero return code."""
    result = host.run(command)
    assert result.rc == 0, (
        f"{description} failed with rc={result.rc}\n"
        f"stdout:\n{result.stdout}\n"
        f"stderr:\n{result.stderr}"
    )
    return result


def check_iperf_open(host, label: str, target: str):
    """Verify that iperf3 can reach the target address."""
    if_command = (
        "if command -v timeout >/dev/null 2>&1; then "
        f"timeout 10 iperf3 -c {target} -t 1 -P 1 --connect-timeout 3000 >/dev/null 2>&1; "
        "elif command -v busybox >/dev/null 2>&1 && busybox timeout >/dev/null 2>&1; then "
        f"busybox timeout -t 10 iperf3 -c {target} -t 1 -P 1 --connect-timeout 3000 >/dev/null 2>&1; "
        "else "
        f"iperf3 -c {target} -t 1 -P 1 --connect-timeout 3000 >/dev/null 2>&1; "
        "fi"
    )
    command = (
        f"{if_command} && echo open || {{ echo closed; false; }}"
    )
    last_result = None
    for attempt in range(1, 4):
        result = host.run(command)
        if result.rc == 0 and "open" in result.stdout.strip():
            return result
        last_result = result
        if attempt < 4:
            time.sleep(2)
    assert last_result is not None
    raise AssertionError(
        f"{label} iperf3 check failed after retries (target {target}).\n"
        f"exit={last_result.rc}\nstdout:\n{last_result.stdout}\n"
        f"stderr:\n{last_result.stderr}"
    )


def ensure_stage_one(
    router_host,
    user: str,
    client_lan: str,
    server_addr: str = "10.0.0.1",
    server_lan: str = "10.0.101.0/24",
    skip_if_active: bool = True,
):
    """
    Execute the stage-one provisioning script for a specific server/client pair.
    Returns the cached CommandResult when the script has already been run for the
    same (server, user, LAN) combination and the tunnel reports as active.
    """
    key = (server_addr, server_lan, user, client_lan)
    cached = _PROVISIONED.get(key)
    if cached is not None and skip_if_active:
        cached_status = router_host.run("x2 status")
        if cached_status.rc == 0 and "tunnel" in cached_status.stdout.lower():
            _PROVISIONED[key] = cached_status
            return cached_status
        _PROVISIONED.pop(key, None)

    if skip_if_active:
        status = router_host.run("x2 status")
        if status.rc == 0 and "tunnel" in status.stdout.lower():
            _PROVISIONED[key] = status
            return status

    command = (
        f"curl -fsSL {SETUP_URL} | XRAY_SKIP_PORT_CHECK=1 sh -s -- "
        f"{server_addr} {user} {server_lan} {client_lan}"
    )
    result = router_host.run(command)
    combined_output = f"{result.stdout}\n{result.stderr}"
    if result.rc != 0:
        lower_output = combined_output.lower()
        if "failed to download" in lower_output or "failed to send request" in lower_output:
            pytest.skip(
                f"xsetup for {user} requires network access to downloads.openwrt.org (skipping test).\n"
                f"stdout:\n{result.stdout}\nstderr:\n{result.stderr}"
            )
        raise AssertionError(
            f"xsetup for {user} failed with rc={result.rc}\n"
            f"stdout:\n{result.stdout}\n"
            f"stderr:\n{result.stderr}"
        )
    assert "All steps completed successfully." in combined_output, (
        "xsetup did not report success.\n"
        f"stdout:\n{result.stdout}\nstderr:\n{result.stderr}"
    )

    _PROVISIONED[key] = result
    return result


__all__ = ["run_checked", "check_iperf_open", "ensure_stage_one"]
