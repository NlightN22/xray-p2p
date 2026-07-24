from __future__ import annotations

import time

import pytest

from tests.host.host_common.polling import wait_until
from tests.host.linux import _helpers as helpers
from tests.host.linux import _heartbeat_acceptance_helpers as heartbeat
from tests.host.linux import _runtime_disable as runtime
from tests.host.linux.flows import tunnel_b_to_a_fixture as fixture
from tests.host.tunnel import common as tunnel_common


pytestmark = [pytest.mark.host, pytest.mark.linux, pytest.mark.destructive]
tunnel_environment = fixture.tunnel_environment
AUX_SERVER_IP = "10.62.10.13"


def test_heartbeat_freshness_transitions_report_and_disabled(tunnel_environment):
    env = tunnel_environment
    server = env["server_host"]
    client = env["client_host"]
    server_runner = env["server_runner"]
    client_runner = env["client_runner"]
    try:
        server_runner("server", "service", "start", check=True)
        client_runner("client", "service", "start", check=True)
        runtime.wait_for_service(server, "server", active=True)
        runtime.wait_for_service(client, "client", active=True)

        initial_client = heartbeat.wait_entry(
            client, helpers.CLIENT_HEARTBEAT_STATE_FILE, "healthy"
        )
        initial_server = heartbeat.wait_entry(
            server, helpers.SERVER_HEARTBEAT_STATE_FILE, "healthy"
        )
        assert initial_client.get("capability") == "xp2p-heartbeat"
        assert initial_server.get("tag") == env["endpoint_tag"]
        manual = client_runner("ping", fixture.SERVER_IP, "--tunnel", "--count", "2", check=True)
        tunnel_common.assert_zero_loss(manual, "manual ping before heartbeat acceptance")

        server_runner("server", "service", "restart", check=True)
        runtime.wait_for_service(server, "server", active=True)
        heartbeat.wait_fresh(
            server, helpers.SERVER_HEARTBEAT_STATE_FILE, initial_server
        )
        heartbeat.wait_fresh(
            client, helpers.CLIENT_HEARTBEAT_STATE_FILE, initial_client
        )

        client_before_restart = heartbeat.wait_entry(
            client, helpers.CLIENT_HEARTBEAT_STATE_FILE, "healthy"
        )
        server_before_client_restart = heartbeat.wait_entry(
            server, helpers.SERVER_HEARTBEAT_STATE_FILE, "healthy"
        )
        client_runner("client", "service", "restart", check=True)
        runtime.wait_for_service(client, "client", active=True)
        heartbeat.wait_fresh(
            client, helpers.CLIENT_HEARTBEAT_STATE_FILE, client_before_restart
        )
        heartbeat.wait_fresh(
            server,
            helpers.SERVER_HEARTBEAT_STATE_FILE,
            server_before_client_restart,
        )

        server_runner("server", "service", "stop", check=True)
        heartbeat.wait_entry(
            client, helpers.CLIENT_HEARTBEAT_STATE_FILE, "unhealthy"
        )
        server_runner("server", "service", "start", check=True)
        runtime.wait_for_service(server, "server", active=True)
        heartbeat.wait_entry(
            client, helpers.CLIENT_HEARTBEAT_STATE_FILE, "healthy"
        )

        heartbeat.force_server_persistence_failure(server)
        try:
            for failures in range(1, 4):
                report = heartbeat.wait_entry(
                    client,
                    helpers.CLIENT_HEARTBEAT_STATE_FILE,
                    "unhealthy" if failures == 3 else "healthy",
                    failure_stage="report",
                    consecutive_failures=failures,
                )
                assert report.get("capability") == "xp2p-heartbeat"
        finally:
            heartbeat.restore_server_heartbeat_file(server)
        heartbeat.wait_entry(
            client, helpers.CLIENT_HEARTBEAT_STATE_FILE, "healthy"
        )

        before_disabled = heartbeat.entry(
            client, helpers.CLIENT_HEARTBEAT_STATE_FILE
        )
        heartbeat.set_heartbeat_mode(client, "disabled")
        client_runner("client", "service", "restart", check=True)
        runtime.wait_for_service(client, "client", active=True)
        time.sleep(6)
        after_disabled = heartbeat.entry(
            client, helpers.CLIENT_HEARTBEAT_STATE_FILE
        )
        assert after_disabled.get("attempts") == before_disabled.get("attempts")
        state = client_runner("client", "state", "--json", "--health-details", check=True)
        row = next(
            row
            for row in tunnel_common.parse_state_result(state.stdout or "")
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
        heartbeat.restore_server_heartbeat_file(server)
        server_runner("server", "service", "stop")
        client_runner("client", "service", "stop")


def test_heartbeat_uses_endpoint_credentials_for_duplicate_user(
    tunnel_environment, aux_host
):
    env = tunnel_environment
    client = env["client_host"]
    client_runner = env["client_runner"]
    aux_runner = runtime.xp2p_runner(aux_host)
    shared_user = env["client_user"]
    try:
        aux_runner(
            "server", "install", "--json",
            "--path", helpers.INSTALL_ROOT.as_posix(),
            "--config-dir", helpers.SERVER_CONFIG_DIR_NAME,
            "--host", AUX_SERVER_IP,
            "--force",
            check=True,
        )
        added = aux_runner(
            "server", "user", "add", "--json",
            "--path", helpers.INSTALL_ROOT.as_posix(),
            "--config-dir", helpers.SERVER_CONFIG_DIR_NAME,
            "--id", shared_user,
            "--host", AUX_SERVER_IP,
            check=True,
        )
        credential = helpers.parse_json_credential(added.stdout or "")
        assert credential["link"], "Expected second endpoint connection link"
        client_runner(
            "client", "install",
            "--path", helpers.INSTALL_ROOT.as_posix(),
            "--config-dir", helpers.CLIENT_CONFIG_DIR_NAME,
            "--mode", "proxy",
            "--link", credential["link"],
            check=True,
        )

        env["server_runner"]("server", "service", "start", check=True)
        aux_runner("server", "service", "start", check=True)
        client_runner("client", "service", "start", check=True)
        runtime.wait_for_service(aux_host, "server", active=True)
        runtime.wait_for_service(client, "client", active=True)

        def both_healthy():
            result = client_runner(
                "client", "state", "--json", "--health-details", check=True
            )
            rows = [
                row for row in tunnel_common.parse_state_result(result.stdout or "")
                if row.get("CLIENT_USER") == shared_user
            ]
            return rows if len(rows) >= 2 and all(
                row.get("STATUS") == "healthy" for row in rows
            ) else None

        wait_until(
            "duplicate-user endpoints to become healthy",
            both_healthy,
            timeout_seconds=45.0,
            poll_interval=1.0,
        )
    except Exception:
        helpers.dump_failure_state(client, "heartbeat-duplicate-user-client")
        helpers.dump_failure_state(aux_host, "heartbeat-duplicate-user-aux")
        raise
    finally:
        aux_runner("server", "service", "stop")
        client_runner("client", "service", "stop")
