from __future__ import annotations

import time

import pytest

from tests.host.host_common.polling import wait_until
from tests.host.linux import _helpers as helpers
from tests.host.linux import _runtime_disable as runtime
from tests.host.linux.flows import tunnel_b_to_a_fixture as fixture


pytestmark = [pytest.mark.host, pytest.mark.linux, pytest.mark.destructive]
tunnel_environment = fixture.tunnel_environment


def test_control_server_listener_closes_before_service_stop_returns(tunnel_environment):
    env = tunnel_environment
    host = env["server_host"]
    runner = env["server_runner"]
    runner("server", "service", "start", check=True)
    runtime.wait_for_service(host, "server", active=True)
    fixture.wait_for_port(host, fixture.SERVER_DIAGNOSTICS_PORT)

    runner("server", "service", "stop", check=True)
    runtime.wait_for_service(host, "server", active=False)
    wait_until(
        "control listener to close after service stop",
        lambda: True
        if host.run(
            f"sudo -n ss -lnt | grep -q ':{fixture.SERVER_DIAGNOSTICS_PORT} '"
        ).rc
        != 0
        else None,
        timeout_seconds=10.0,
        poll_interval=0.25,
    )


def test_network_setup_and_xray_workers_exit_during_immediate_stop(tunnel_environment):
    env = tunnel_environment
    host = env["client_host"]
    runner = env["client_runner"]
    runner(
        "client",
        "mode",
        "tun",
        "--path",
        helpers.INSTALL_ROOT.as_posix(),
        "--config-dir",
        helpers.CLIENT_CONFIG_DIR_NAME,
        check=True,
    )

    started = time.monotonic()
    result = host.run(
        "sudo -n /bin/sh -c "
        "'systemctl start xp2p-client.service & starter=$!; "
        "sleep 0.1; systemctl stop xp2p-client.service; wait $starter || true'"
    )
    elapsed = time.monotonic() - started
    assert result.rc == 0, result.stderr
    assert elapsed < 15.0, f"service stop exceeded lifecycle deadline: {elapsed:.2f}s"
    runtime.wait_for_service(host, "client", active=False)
    assert host.run("pgrep -x xray >/dev/null").rc != 0
    assert host.run("pgrep -f '/usr/bin/[x]p2p client run' >/dev/null").rc != 0
