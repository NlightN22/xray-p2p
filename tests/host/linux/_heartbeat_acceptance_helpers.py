from __future__ import annotations

import json

from tests.host.host_common.polling import wait_until
from tests.host.linux import _helpers as helpers


def entry(host, path):
    result = host.run(f"cat {path}")
    if result.rc != 0:
        return None
    entries = list((json.loads(result.stdout or "{}").get("entries") or {}).values())
    return max(entries, key=lambda item: item.get("last_seen") or "") if entries else None


def wait_entry(
    host,
    path,
    status,
    *,
    failure_stage=None,
    consecutive_failures=None,
):
    def poll():
        current = entry(host, path)
        if not current:
            return None
        if status is not None and current.get("status") != status:
            return None
        if (
            failure_stage is not None
            and current.get("failure_stage") != failure_stage
        ):
            return None
        if (
            consecutive_failures is not None
            and current.get("consecutive_failures") != consecutive_failures
        ):
            return None
        return current

    return wait_until(
        f"heartbeat {status or failure_stage} in {path}",
        poll,
        timeout_seconds=45.0,
        poll_interval=1.0,
    ).value


def wait_fresh(host, path, baseline):
    def poll():
        current = entry(host, path)
        if current and (current.get("last_seen"), current.get("attempts")) != (
            baseline.get("last_seen"),
            baseline.get("attempts"),
        ):
            return current
        return None

    return wait_until(
        f"fresh heartbeat in {path}",
        poll,
        timeout_seconds=45.0,
        poll_interval=1.0,
    ).value


def force_server_persistence_failure(host):
    path = helpers.SERVER_HEARTBEAT_STATE_FILE
    backup = f"{path}.acceptance-backup"
    result = host.run(
        f"rm -rf {backup}; mv {path} {backup}; mkdir {path}; touch {path}/block"
    )
    assert result.rc == 0, result.stderr


def restore_server_heartbeat_file(host):
    path = helpers.SERVER_HEARTBEAT_STATE_FILE
    backup = f"{path}.acceptance-backup"
    host.run(
        f"if [ -d {path} ]; then rm -rf {path}; fi; "
        f"if [ -f {backup} ]; then mv {backup} {path}; fi"
    )


def set_heartbeat_mode(host, mode: str):
    path = helpers.CLIENT_CONFIG_FILE
    result = host.run(
        f"sed -i -E 's/heartbeat_mode = "
        f"\"(auto|required|disabled)\"/heartbeat_mode = \"{mode}\"/' {path}"
    )
    assert result.rc == 0, result.stderr
