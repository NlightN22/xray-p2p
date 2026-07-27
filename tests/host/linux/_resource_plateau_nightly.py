from __future__ import annotations

from contextlib import contextmanager
import shlex
import time

import pytest

from tests.host.linux import _helpers as helpers
from tests.host.linux import _resource_plateau as plateau
from tests.host.linux.flows import tunnel_b_to_a_fixture as fixture

BRIDGE = "xp2p-plateau"
NETWORK = "10.63.0.0/24"
GATEWAY = "10.63.0.1"
EXTRA_CLIENTS = 3


@contextmanager
def extra_client_sessions(env: dict, host):
    _create_network(env, host)
    sessions = []
    try:
        for index in range(EXTRA_CLIENTS):
            sessions.append(_start_client(env, host, index))
        yield sessions
    finally:
        for session in reversed(sessions):
            host.run(f"sudo -n kill {session['pid']} >/dev/null 2>&1 || true")
        _wait_for_shutdown(env, host, sessions)
        _remove_network(env, host)


def _create_network(env: dict, host) -> None:
    _remove_network(env, host)
    commands = [
        f"sudo -n ip link add {BRIDGE} type bridge",
        f"sudo -n ip addr add {GATEWAY}/24 dev {BRIDGE}",
        f"sudo -n ip link set {BRIDGE} up",
        "sudo -n sysctl -w net.ipv4.ip_forward=1 >/dev/null",
    ]
    for index in range(EXTRA_CLIENTS):
        ns = _namespace(index)
        host_veth = f"xp2ph{index}"
        commands.extend(
            [
                f"sudo -n ip netns add {ns}",
                f"sudo -n ip link add {host_veth} type veth peer name eth0 netns {ns}",
                f"sudo -n ip link set {host_veth} master {BRIDGE}",
                f"sudo -n ip link set {host_veth} up",
                f"sudo -n ip netns exec {ns} ip addr add 10.63.0.{10 + index}/24 dev eth0",
                f"sudo -n ip netns exec {ns} ip link set lo up",
                f"sudo -n ip netns exec {ns} ip link set eth0 up",
                f"sudo -n ip netns exec {ns} ip route add default via {GATEWAY}",
            ]
        )
    for command in commands:
        result = host.run(command)
        if result.rc != 0:
            pytest.fail(f"nightly client network setup failed: {command}\n{result.stderr}")
    route = env["server_host"].run(
        f"sudo -n ip route replace {NETWORK} via 10.62.10.13"
    )
    if route.rc != 0:
        pytest.fail(f"nightly client server route failed: {route.stderr}")


def _start_client(env: dict, host, index: int) -> dict:
    ns = _namespace(index)
    root = f"/tmp/xp2p-plateau-client-{index}"
    metrics = f"{root}/runtime.metrics"
    credential = env["server_runner"](
        "server", "user", "add", "--json",
        "--path", env["server_install_path"],
        "--config-dir", helpers.SERVER_CONFIG_DIR_NAME,
        "--id", f"resource-plateau-nightly-{index}@example.com",
        "--host", fixture.SERVER_IP,
        check=True,
    )
    link = helpers.parse_json_credential(credential.stdout or "")["link"]
    quoted_link = shlex.quote(link)
    setup = (
        f"sudo -n rm -rf {root}; sudo -n mkdir -p {root}/bin {root}/logs; "
        f"sudo -n ln -s /etc/xp2p/bin/xray {root}/bin/xray; "
        f"sudo -n ip netns exec {ns} env XP2P_CONFIG_ROOT={root} XP2P_LOG_ROOT={root}/logs "
        f"/usr/bin/xp2p client install --path {root} --config-dir config-client "
        f"--mode proxy --link {quoted_link} --force"
    )
    result = host.run(setup)
    if result.rc != 0:
        pytest.fail(f"nightly client {index} install failed: {result.stderr}")
    start = (
        f"sudo -n ip netns exec {ns} env XP2P_CONFIG_ROOT={root} XP2P_LOG_ROOT={root}/logs "
        f"XP2P_RUNTIME_METRICS_FILE={metrics} XP2P_RUNTIME_METRICS_INTERVAL=1s "
        f"nohup /usr/bin/xp2p client run --path {root} --config-dir config-client "
        f"--auto-install --quiet >/tmp/xp2p-plateau-client-{index}.log 2>&1 & echo $!"
    )
    result = host.run(start)
    wrapper_pid = int((result.stdout or "").strip())
    pid = _wait_for_metrics_pid(host, index, wrapper_pid, metrics)
    xray_pid = _wait_for_xray(host, index, pid, root)
    return {
        "host": host,
        "pid": pid,
        "xray_pid": xray_pid,
        "runtime_metrics": metrics,
        "peer": fixture.SERVER_IP,
        "source_ip": f"10.63.0.{10 + index}",
    }


def _wait_for_metrics_pid(host, index: int, wrapper_pid: int, metrics: str) -> int:
    deadline = time.monotonic() + 15.0
    while time.monotonic() < deadline:
        if host.run(f"sudo -n kill -0 {wrapper_pid}").rc != 0:
            pytest.fail(f"nightly client {index} exited during startup")
        result = host.run(
            f"sudo -n awk -F= '$1 == \"pid\" {{print $2}}' {metrics}"
        )
        if result.rc == 0 and (result.stdout or "").strip():
            return int(result.stdout.strip())
        time.sleep(0.5)
    pytest.fail(f"nightly client {index} runtime metrics were not published")


def _wait_for_xray(host, index: int, pid: int, root: str) -> int:
    deadline = time.monotonic() + 15.0
    while time.monotonic() < deadline:
        if host.run(f"sudo -n kill -0 {pid}").rc != 0:
            pytest.fail(f"nightly client {index} exited during startup")
        xray = host.run(f"sudo -n pgrep -f '{root}/bin/[x]ray' | head -n1")
        if xray.rc == 0 and (xray.stdout or "").strip():
            return int(xray.stdout.strip())
        time.sleep(0.5)
    log = host.run(f"sudo -n tail -n 100 /tmp/xp2p-plateau-client-{index}.log")
    pytest.fail(
        f"nightly client {index} xray child was not found\n{log.stdout}\n{log.stderr}"
    )


def _wait_for_shutdown(env: dict, host, sessions: list[dict]) -> None:
    deadline = time.monotonic() + 15.0
    while time.monotonic() < deadline:
        alive = [
            pid for session in sessions for pid in (session["pid"], session["xray_pid"])
            if host.run(f"sudo -n test -e /proc/{pid}").rc == 0
        ]
        connections = {
            session["source_ip"]: plateau.host_peer_tcp(env["server_host"], session["source_ip"])
            for session in sessions
        }
        if not alive and not any(connections.values()):
            return
        time.sleep(0.5)
    pytest.fail(
        f"nightly owners did not release resources: alive={alive}, connections={connections}"
    )


def _remove_network(env: dict, host) -> None:
    env["server_host"].run(f"sudo -n ip route del {NETWORK} >/dev/null 2>&1 || true")
    for index in range(EXTRA_CLIENTS):
        host.run(f"sudo -n ip netns del {_namespace(index)} >/dev/null 2>&1 || true")
    host.run(f"sudo -n ip link del {BRIDGE} >/dev/null 2>&1 || true")


def _namespace(index: int) -> str:
    return f"xp2p-plateau-{index}"
