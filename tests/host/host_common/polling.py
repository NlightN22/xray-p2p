from __future__ import annotations

import time
from collections.abc import Callable
from dataclasses import dataclass
from typing import Generic, TypeVar

T = TypeVar("T")


@dataclass(frozen=True, slots=True)
class PollResult(Generic[T]):
    value: T
    attempts: int
    elapsed_seconds: float


def wait_until(
    description: str,
    predicate: Callable[[], T | None],
    *,
    timeout_seconds: float,
    poll_interval: float,
) -> PollResult[T]:
    deadline = time.monotonic() + timeout_seconds
    attempts = 0
    last_error: BaseException | None = None
    start = time.monotonic()
    while time.monotonic() < deadline:
        attempts += 1
        try:
            value = predicate()
        except BaseException as exc:
            last_error = exc
            value = None
        if value is not None:
            return PollResult(value=value, attempts=attempts, elapsed_seconds=time.monotonic() - start)
        time.sleep(poll_interval)
    error_hint = f" Last error: {last_error!r}" if last_error else ""
    raise TimeoutError(
        f"Timed out after {timeout_seconds:.1f}s waiting for {description}.{error_hint}"
    )

