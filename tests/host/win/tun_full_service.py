from __future__ import annotations

import time

import pytest

from tests.host.win import env as _env

SERVICE_TIMEOUT = 90.0
POLL_INTERVAL = 2.0
APPLY_REQUEST = _env.CONFIG_ROOT / _env.APPLY_DIR_NAME / "apply.request"


def require_client_service(host) -> None:
    if not _env.service_exists(host, "xp2p-client"):
        pytest.skip("xp2p-client service is not registered; MSI install required.")


def wait_for_service_state(runner, expected_active: bool) -> None:
    wait_for_role_service_state(runner, "client", expected_active)


def start_client_service(runner) -> None:
    start_service(runner, "client")


def stop_client_service(runner) -> None:
    stop_service(runner, "client")


def wait_for_role_service_state(runner, role: str, expected_active: bool) -> None:
    deadline = time.time() + SERVICE_TIMEOUT
    last_stdout = ""
    last_stderr = ""
    while time.time() < deadline:
        result = runner(role, "service", "status")
        last_stdout = result.stdout or ""
        last_stderr = result.stderr or ""
        active = result.rc == 0
        if active == expected_active:
            return
        time.sleep(POLL_INTERVAL)
    state = "active" if expected_active else "inactive"
    pytest.fail(
        f"xp2p {role} service did not reach {state} state.\n"
        f"STDOUT:\n{last_stdout}\nSTDERR:\n{last_stderr}"
    )


def wait_for_apply_request_clear(host, timeout: float = 90.0) -> None:
    deadline = time.time() + timeout
    while time.time() < deadline:
        if not _env.path_exists(host, APPLY_REQUEST):
            return
        time.sleep(POLL_INTERVAL)
    pytest.fail(f"apply.request did not clear after {timeout} seconds.")


def start_service(runner, role: str) -> None:
    runner(role, "service", "start", check=True)
    wait_for_role_service_state(runner, role, expected_active=True)


def stop_service(runner, role: str) -> None:
    runner(role, "service", "stop", check=True)
    wait_for_role_service_state(runner, role, expected_active=False)
