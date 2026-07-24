from __future__ import annotations

import json

from . import _bare_xray as bare
from . import _helpers as helpers
from tests.host.host_common.polling import wait_until


def assert_status(
    client_host,
    status: str,
    mode: str,
    *,
    check: str | None = None,
    failure_stage: str | None = None,
) -> None:
    def observed():
        result = client_host.run(f"cat {helpers.CLIENT_HEARTBEAT_STATE_FILE}")
        if result.rc != 0:
            return None
        entries = (json.loads(result.stdout or "{}").get("entries") or {}).values()
        return next(
            (
                entry
                for entry in entries
                if entry.get("host") == bare.TLS_NAME
                and entry.get("status") == status
                and entry.get("mode") == mode
                and (check is None or entry.get("capability") == check)
                and (
                    failure_stage is None
                    or entry.get("failure_stage") == failure_stage
                )
            ),
            None,
        )

    try:
        wait_until(
            f"bare Xray heartbeat status {status}",
            observed,
            timeout_seconds=30.0,
            poll_interval=1.0,
        )
    except TimeoutError:
        bare.failure_dump(client_host, client_host)
        raise


def assert_failure_threshold(
    client_host,
    *,
    check: str,
    before_status: str,
    threshold_status: str,
    failure_stage: str | None = None,
) -> None:
    for failures in range(1, 4):
        expected_status = threshold_status if failures == 3 else before_status

        def observed():
            result = client_host.run(f"cat {helpers.CLIENT_HEARTBEAT_STATE_FILE}")
            if result.rc != 0:
                return None
            entries = (
                json.loads(result.stdout or "{}").get("entries") or {}
            ).values()
            return next(
                (
                    entry
                    for entry in entries
                    if entry.get("host") == bare.TLS_NAME
                    and entry.get("capability", "unknown") == check
                    and entry.get("status") == expected_status
                    and entry.get("consecutive_failures") == failures
                    and (
                        failure_stage is None
                        or entry.get("failure_stage") == failure_stage
                    )
                ),
                None,
            )

        wait_until(
            f"{check} failure {failures} to be {expected_status}",
            observed,
            timeout_seconds=30.0,
            poll_interval=0.2,
        )


def wait_entry(
    host,
    endpoint_host: str,
    status: str,
    check: str,
    *,
    path=helpers.CLIENT_HEARTBEAT_STATE_FILE,
):
    def observed():
        result = host.run(f"cat {path}")
        if result.rc != 0:
            return None
        entries = (json.loads(result.stdout or "{}").get("entries") or {}).values()
        return next(
            (
                entry
                for entry in entries
                if entry.get("host") == endpoint_host
                and entry.get("status") == status
                and entry.get("capability") == check
            ),
            None,
        )

    return wait_until(
        f"{endpoint_host} heartbeat {check}/{status}",
        observed,
        timeout_seconds=45.0,
        poll_interval=0.5,
    ).value
