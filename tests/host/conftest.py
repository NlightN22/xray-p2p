"""
Host-level pytest configuration.

Shared fixtures are defined inside suite-specific subpackages
(`tests.host.win`, `tests.host.linux`, etc.). This file is intentionally
minimal so that platform-specific logic lives alongside the respective tests.
"""

from datetime import datetime, timezone


def _utc_timestamp() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def pytest_runtest_logstart(nodeid: str, location) -> None:
    print(f"TEST START {_utc_timestamp()} {nodeid}", flush=True)


def pytest_runtest_logfinish(nodeid: str, location) -> None:
    print(f"TEST END {_utc_timestamp()} {nodeid}", flush=True)
