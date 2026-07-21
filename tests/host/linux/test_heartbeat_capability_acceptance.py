from __future__ import annotations

import json
import time

import pytest

from tests.host.host_common.polling import wait_until
from tests.host.linux import _helpers as helpers
from tests.host.linux.flows import tunnel_b_to_a_fixture as fixture
from tests.host.tunnel import common as tunnel_common


pytestmark = [pytest.mark.host, pytest.mark.linux, pytest.mark.destructive]
tunnel_environment = fixture.tunnel_environment


def test_heartbeat_freshness_transitions_report_and_disabled(tunnel_environment):
    env = tunnel_environment
    server = env["server_host"]
    client = env["client_host"]
    server_runner = env["server_runner"]
    client_runner = env["client_runner"]
    try:
        server_runner("server", "service", "start", check=True)
        client_runner("client", "service", "start", check=True)
        helpers.wait_for_service_state(server, "server", expected_active=True)
        helpers.wait_for_service_state(client, "client", expected_active=True)

        initial_client = _wait_entry(client, helpers.CLIENT_HEARTBEAT_STATE_FILE, "healthy")
        initial_server = _wait_entry(server, helpers.SERVER_HEARTBEAT_STATE_FILE, "healthy")
        manual = client_runner("ping", fixture.SERVER_IP, "--tunnel", "--count", "2", check=True)
        tunnel_common.assert_zero_loss(manual, "manual ping before heartbeat acceptance")

        server_runner("server", "service", "restart", check=True)
        helpers.wait_for_service_state(server, "server", expected_active=True)
        _wait_fresh(server, helpers.SERVER_HEARTBEAT_STATE_FILE, initial_server)
        _wait_fresh(client, helpers.CLIENT_HEARTBEAT_STATE_FILE, initial_client)

        client_before_restart = _wait_entry(
            client, helpers.CLIENT_HEARTBEAT_STATE_FILE, "healthy"
        )
        server_before_client_restart = _wait_entry(
            server, helpers.SERVER_HEARTBEAT_STATE_FILE, "healthy"
        )
        client_runner("client", "service", "restart", check=True)
        helpers.wait_for_service_state(client, "client", expected_active=True)
        _wait_fresh(
            client, helpers.CLIENT_HEARTBEAT_STATE_FILE, client_before_restart
        )
        _wait_fresh(
            server,
            helpers.SERVER_HEARTBEAT_STATE_FILE,
            server_before_client_restart,
        )

        server_runner("server", "service", "stop", check=True)
        _wait_entry(client, helpers.CLIENT_HEARTBEAT_STATE_FILE, "unhealthy")
        server_runner("server", "service", "start", check=True)
        helpers.wait_for_service_state(server, "server", expected_active=True)
        _wait_entry(client, helpers.CLIENT_HEARTBEAT_STATE_FILE, "healthy")

        _force_server_persistence_failure(server)
        try:
            report = _wait_entry(
                client,
                helpers.CLIENT_HEARTBEAT_STATE_FILE,
                None,
                failure_stage="report",
            )
            assert report.get("capability") == "detected"
        finally:
            _restore_server_heartbeat_file(server)
        _wait_entry(client, helpers.CLIENT_HEARTBEAT_STATE_FILE, "healthy")

        before_disabled = _entry(client, helpers.CLIENT_HEARTBEAT_STATE_FILE)
        _set_heartbeat_mode(client, "disabled")
        client_runner("client", "service", "restart", check=True)
        helpers.wait_for_service_state(client, "client", expected_active=True)
        time.sleep(6)
        after_disabled = _entry(client, helpers.CLIENT_HEARTBEAT_STATE_FILE)
        assert after_disabled.get("attempts") == before_disabled.get("attempts")
        state = client_runner("client", "state", check=True)
        row = next(
            row
            for row in tunnel_common.parse_state_rows(state.stdout or "")
            if row.get("TAG") == env["endpoint_tag"]
        )
        assert row["STATUS"] == "disabled"
        assert row["MODE"] == "disabled"
        assert row["CHECK"] == "none"
    except Exception:
        helpers.dump_failure_state(client, "heartbeat-capability-client")
        helpers.dump_failure_state(server, "heartbeat-capability-server")
        raise
    finally:
        _restore_server_heartbeat_file(server)
        server_runner("server", "service", "stop")
        client_runner("client", "service", "stop")


def _entry(host, path):
    result = host.run(f"cat {path}")
    if result.rc != 0:
        return None
    entries = list((json.loads(result.stdout or "{}").get("entries") or {}).values())
    return max(entries, key=lambda item: item.get("last_seen") or "") if entries else None


def _wait_entry(host, path, status, *, failure_stage=None):
    def poll():
        entry = _entry(host, path)
        if not entry:
            return None
        if status is not None and entry.get("status") != status:
            return None
        if failure_stage is not None and entry.get("failure_stage") != failure_stage:
            return None
        return entry

    return wait_until(
        f"heartbeat {status or failure_stage} in {path}",
        poll,
        timeout_seconds=45.0,
        poll_interval=1.0,
    ).value


def _wait_fresh(host, path, baseline):
    def poll():
        current = _entry(host, path)
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


def _force_server_persistence_failure(host):
    path = helpers.SERVER_HEARTBEAT_STATE_FILE
    backup = f"{path}.acceptance-backup"
    result = host.run(f"rm -rf {backup}; mv {path} {backup}; mkdir {path}; touch {path}/block")
    assert result.rc == 0, result.stderr


def _restore_server_heartbeat_file(host):
    path = helpers.SERVER_HEARTBEAT_STATE_FILE
    backup = f"{path}.acceptance-backup"
    host.run(f"if [ -d {path} ]; then rm -rf {path}; fi; if [ -f {backup} ]; then mv {backup} {path}; fi")


def _set_heartbeat_mode(host, mode: str):
    path = helpers.CLIENT_CONFIG_FILE
    result = host.run(
        f"sed -i -E 's/heartbeat_mode = \"(auto|required|disabled)\"/heartbeat_mode = \"{mode}\"/' {path}"
    )
    assert result.rc == 0, result.stderr
