from __future__ import annotations

import time
from pathlib import Path

import pytest
from testinfra.host import Host

from tests.host.win import env as win_env

DEFAULT_POLL_SECONDS = 1.0


def apply_request_path() -> Path:
    return win_env.CONFIG_ROOT / win_env.APPLY_DIR_NAME / "apply.request"


def apply_error_path() -> Path:
    return win_env.CONFIG_ROOT / win_env.APPLY_DIR_NAME / "apply.error"


def wait_for_path_absent(
    host: Host,
    path: Path,
    *,
    timeout: float,
    poll_seconds: float = DEFAULT_POLL_SECONDS,
    description: str | None = None,
    dump_label: str | None = None,
) -> None:
    deadline = time.monotonic() + float(timeout)
    while time.monotonic() < deadline:
        if not win_env.path_exists(host, path):
            return
        time.sleep(poll_seconds)

    dump_path = None
    if dump_label:
        try:
            dump_path = win_env.dump_failure_state(host, label=dump_label)
        except Exception as exc:  # noqa: BLE001
            print(f"WARNING: failed to dump failure state ({dump_label}): {exc}")

    message = description or f"{path.name} did not clear"
    if dump_path:
        message = f"{message}.\nFailure dump: {dump_path}"
    pytest.fail(f"{message} after {timeout} seconds.\nPath: {path}")


def wait_for_path_present(
    host: Host,
    path: Path,
    *,
    timeout: float,
    poll_seconds: float = DEFAULT_POLL_SECONDS,
    description: str | None = None,
    dump_label: str | None = None,
) -> None:
    deadline = time.monotonic() + float(timeout)
    while time.monotonic() < deadline:
        if win_env.path_exists(host, path):
            return
        time.sleep(poll_seconds)

    dump_path = None
    if dump_label:
        try:
            dump_path = win_env.dump_failure_state(host, label=dump_label)
        except Exception as exc:  # noqa: BLE001
            print(f"WARNING: failed to dump failure state ({dump_label}): {exc}")

    message = description or f"{path.name} did not appear"
    if dump_path:
        message = f"{message}.\nFailure dump: {dump_path}"
    pytest.fail(f"{message} after {timeout} seconds.\nPath: {path}")


def wait_for_apply_request_clear(
    host: Host,
    *,
    timeout: float = 90.0,
    poll_seconds: float = DEFAULT_POLL_SECONDS,
    dump_label: str | None = None,
) -> None:
    wait_for_path_absent(
        host,
        apply_request_path(),
        timeout=timeout,
        poll_seconds=poll_seconds,
        description="apply.request did not clear",
        dump_label=dump_label,
    )


def wait_for_apply_request_set(
    host: Host,
    *,
    timeout: float = 60.0,
    poll_seconds: float = DEFAULT_POLL_SECONDS,
    dump_label: str | None = None,
) -> None:
    wait_for_path_present(
        host,
        apply_request_path(),
        timeout=timeout,
        poll_seconds=poll_seconds,
        description="apply.request did not appear",
        dump_label=dump_label,
    )


def wait_for_apply_error_clear(
    host: Host,
    *,
    timeout: float = 30.0,
    poll_seconds: float = DEFAULT_POLL_SECONDS,
    dump_label: str | None = None,
) -> None:
    wait_for_path_absent(
        host,
        apply_error_path(),
        timeout=timeout,
        poll_seconds=poll_seconds,
        description="apply.error did not clear",
        dump_label=dump_label,
    )


def wait_for_apply_error_set(
    host: Host,
    *,
    timeout: float = 60.0,
    poll_seconds: float = DEFAULT_POLL_SECONDS,
    dump_label: str | None = None,
) -> None:
    wait_for_path_present(
        host,
        apply_error_path(),
        timeout=timeout,
        poll_seconds=poll_seconds,
        description="apply.error did not appear",
        dump_label=dump_label,
    )
