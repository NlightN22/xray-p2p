from __future__ import annotations

from contextlib import contextmanager
import re
import shlex
import time

import pytest
from testinfra.host import Host

from tests.host.host_common.polling import wait_until


DEFAULT_DEGRADATION = "delay 250ms 120ms 25% loss 8% reorder 10% 25% limit 1000"
PING_SUMMARY_RE = re.compile(r"sent\s*=\s*(?P<sent>\d+),\s*received\s*=\s*(?P<received>\d+)", re.IGNORECASE)


def require_netem_opt_in(env_value: str | None) -> None:
    if env_value != "1":
        pytest.skip("set XP2P_RUN_HEARTBEAT_STORM_TESTS=1 to run netem heartbeat storm lab")


@contextmanager
def netem_degradation(host: Host, peer_ip: str, spec: str = DEFAULT_DEGRADATION):
    dev = route_device(host, peer_ip)
    apply_netem(host, dev, spec)
    try:
        yield dev
    finally:
        clear_netem(host, dev)


def route_device(host: Host, peer_ip: str) -> str:
    quoted = shlex.quote(peer_ip)
    result = host.run(f"ip route get {quoted} | awk '{{for (i=1; i<=NF; i++) if ($i == \"dev\") print $(i+1)}}' | head -n1")
    dev = (result.stdout or "").strip()
    if result.rc != 0 or not dev:
        pytest.fail(
            f"failed to resolve route device for {peer_ip} on {host.backend.hostname}\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return dev


def apply_netem(host: Host, dev: str, spec: str) -> None:
    cmd = f"sudo -n tc qdisc replace dev {shlex.quote(dev)} root netem {spec}"
    result = host.run(cmd)
    if result.rc != 0:
        pytest.fail(
            f"failed to apply netem on {host.backend.hostname}:{dev}\n"
            f"CMD: {cmd}\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )


def clear_netem(host: Host, dev: str) -> None:
    host.run(f"sudo -n tc qdisc del dev {shlex.quote(dev)} root >/dev/null 2>&1 || true")


def wait_for_no_netem(host: Host, peer_ip: str) -> None:
    dev = route_device(host, peer_ip)

    def _poll():
        result = host.run(f"sudo -n tc qdisc show dev {shlex.quote(dev)}")
        if result.rc != 0:
            return None
        output = (result.stdout or "").lower()
        return True if "netem" not in output else None

    try:
        wait_until(
            f"netem qdisc to be removed from {host.backend.hostname}:{dev}",
            _poll,
            timeout_seconds=10.0,
            poll_interval=0.5,
        )
    except TimeoutError as exc:
        pytest.fail(str(exc))


def socket_snapshot(host: Host, peer_ip: str, peer_port: int) -> dict[str, int]:
    quoted_ip = shlex.quote(peer_ip)
    command = (
        "sudo -n /bin/sh -c "
        + shlex.quote(
            "ss -tan | awk -v ip=\"$1\" -v port=\":$2\" "
            "'NR > 1 && ($4 ~ port || $5 ~ port) && ($4 ~ ip || $5 ~ ip) {state[$1]++; total++} "
            "END {print \"total=\" total+0; for (name in state) print name \"=\" state[name]}'"
        )
        + f" -- {quoted_ip} {int(peer_port)}"
    )
    result = host.run(command)
    if result.rc != 0:
        pytest.fail(
            f"failed to collect socket snapshot on {host.backend.hostname}\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return _parse_key_values(result.stdout)


def xray_fd_count(host: Host) -> int:
    command = (
        "sudo -n /bin/sh -c "
        + shlex.quote(
            "pid=$(pgrep -f '/etc/xp2p/bin/[x]ray' | head -n1 || true); "
            "if [ -z \"$pid\" ]; then echo 0; exit 0; fi; "
            "find \"/proc/$pid/fd\" -maxdepth 1 -type l 2>/dev/null | wc -l"
        )
    )
    result = host.run(command)
    if result.rc != 0:
        pytest.fail(
            f"failed to collect xray fd count on {host.backend.hostname}\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return int((result.stdout or "0").strip() or "0")


def run_heartbeat_probe_burst(env: dict, *, attempts: int, timeout_seconds: int) -> list[dict[str, str | int]]:
    results = []
    for _ in range(attempts):
        result = env["client_runner"](
            "ping",
            env["server_ip"],
            "--tunnel",
            "--count",
            "1",
            "--timeout",
            str(timeout_seconds),
            check=False,
        )
        results.append(
            {
                "rc": result.rc,
                "stdout": result.stdout or "",
                "stderr": result.stderr or "",
            }
        )
        time.sleep(0.2)
    return results


def received_ping_replies(output: str | None) -> int:
    match = PING_SUMMARY_RE.search(output or "")
    if not match:
        return 0
    return int(match.group("received"))


def _parse_key_values(output: str | None) -> dict[str, int]:
    values: dict[str, int] = {}
    for raw in (output or "").splitlines():
        key, sep, value = raw.strip().partition("=")
        if not sep:
            continue
        try:
            values[key] = int(value)
        except ValueError:
            continue
    values.setdefault("total", 0)
    return values
