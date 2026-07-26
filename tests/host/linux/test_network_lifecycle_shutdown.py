from __future__ import annotations

import base64
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


def test_running_xray_exits_before_service_stop_returns(tunnel_environment):
    env = tunnel_environment
    host = env["server_host"]
    runner = env["server_runner"]
    runner("server", "service", "start", check=True)
    runtime.wait_for_service(host, "server", active=True)
    pid = wait_until(
        "xray process to start",
        lambda: _xray_pid(host),
        timeout_seconds=15.0,
        poll_interval=0.25,
    ).value

    started = time.monotonic()
    runner("server", "service", "stop", check=True)
    elapsed = time.monotonic() - started

    assert elapsed < 15.0, f"service stop exceeded lifecycle deadline: {elapsed:.2f}s"
    runtime.wait_for_service(host, "server", active=False)
    assert host.run(f"test ! -e /proc/{pid}").rc == 0
    assert host.run("pgrep -x xray >/dev/null").rc != 0
    assert host.run("pgrep -f '/usr/bin/[x]p2p server run' >/dev/null").rc != 0


def test_active_network_setup_command_exits_during_stop(tunnel_environment):
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

    marker = "/tmp/xp2p-network-setup-started"
    wrapper_dir = "/tmp/xp2p-lifecycle-bin"
    drop_in_dir = "/run/systemd/system/xp2p-client.service.d"
    wrapper = f"""#!/bin/sh
touch {marker}
trap 'exit 130' TERM INT
while :; do sleep 1; done
"""
    encoded_wrapper = base64.b64encode(wrapper.encode()).decode()
    setup = host.run(
        "sudo -n /bin/sh -c "
        f"'rm -f {marker}; mkdir -p {wrapper_dir}; "
        f"printf %s {encoded_wrapper} | base64 -d > {wrapper_dir}/ip; "
        f"chmod 0755 {wrapper_dir}/ip; "
        f"mkdir -p {drop_in_dir}; "
        f"printf \"[Service]\\nEnvironment=PATH={wrapper_dir}:/usr/sbin:/usr/bin:/sbin:/bin\\n\" "
        f"> {drop_in_dir}/lifecycle-test.conf; systemctl daemon-reload'"
    )
    assert setup.rc == 0, setup.stderr

    started = time.monotonic()
    try:
        result = host.run(
            "sudo -n /bin/sh -c "
            f"'systemctl start xp2p-client.service & starter=$!; "
            f"found=0; for attempt in $(seq 1 100); do "
            f"[ -e {marker} ] && found=1 && break; sleep 0.1; done; "
            "systemctl stop xp2p-client.service; wait $starter || true; "
            "[ $found -eq 1 ]'"
        )
    finally:
        host.run(
            "sudo -n /bin/sh -c "
            f"'rm -rf {wrapper_dir} {marker} {drop_in_dir}; systemctl daemon-reload'"
        )
    elapsed = time.monotonic() - started
    assert result.rc == 0, (
        "network setup did not reach a running ip command before cancellation: "
        f"{result.stderr}"
    )
    assert elapsed < 15.0, f"service stop exceeded lifecycle deadline: {elapsed:.2f}s"
    runtime.wait_for_service(host, "client", active=False)
    assert host.run("pgrep -x xray >/dev/null").rc != 0
    assert host.run("pgrep -f '/usr/bin/[x]p2p client run' >/dev/null").rc != 0


def _xray_pid(host):
    result = host.run("pgrep -x xray | head -n1")
    value = result.stdout.strip()
    return value if result.rc == 0 and value else None
