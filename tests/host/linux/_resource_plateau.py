from __future__ import annotations

from dataclasses import dataclass
import json
from pathlib import Path
import shlex
import time

from testinfra.host import Host


@dataclass(frozen=True)
class PlateauLimit:
    maximum_range: float
    maximum_slope: float


def assess(values: list[int], limit: PlateauLimit) -> dict[str, float]:
    if len(values) < 3:
        raise ValueError("at least three samples are required")
    count = len(values)
    x_mean = (count - 1) / 2
    y_mean = sum(values) / count
    denominator = sum((index - x_mean) ** 2 for index in range(count))
    slope = sum((index - x_mean) * (value - y_mean) for index, value in enumerate(values)) / denominator
    result = {"range": float(max(values) - min(values)), "slope": slope}
    if result["range"] > limit.maximum_range or result["slope"] > limit.maximum_slope:
        raise AssertionError(
            f"resource did not plateau: values={values}, range={result['range']:.3f}, "
            f"slope={result['slope']:.3f}"
        )
    return result


def process_sample(host: Host, pid: int, peer_ip: str) -> dict[str, int]:
    script = """
pid=$1
peer=$2
read rss threads < <(awk '/VmRSS:/ {rss=$2} /Threads:/ {threads=$2} END {print rss+0, threads+0}' /proc/$pid/status)
fd=$(find /proc/$pid/fd -maxdepth 1 -type l 2>/dev/null | wc -l)
socket_fd=$(find /proc/$pid/fd -maxdepth 1 -type l -printf '%l\n' 2>/dev/null | grep -c '^socket:' || true)
established=$(ss -Htan state established | awk -v peer="$peer" '$4 ~ peer || $5 ~ peer {count++} END {print count+0}')
cgroup=$(awk '$1 == "0" {print $3}' /proc/$pid/cgroup)
memory=0
if [ -n "$cgroup" ] && [ -r "/sys/fs/cgroup${cgroup}/memory.current" ]; then
  memory=$(cat "/sys/fs/cgroup${cgroup}/memory.current")
fi
printf 'rss_kib=%s\nthreads=%s\nfd=%s\nsocket_fd=%s\nestablished=%s\ncgroup_memory=%s\n' \
  "$rss" "$threads" "$fd" "$socket_fd" "$established" "$memory"
"""
    command = "sudo -n bash -c " + shlex.quote(script) + f" -- {int(pid)} {shlex.quote(peer_ip)}"
    result = host.run(command)
    if result.rc != 0:
        raise AssertionError(
            f"resource sampling failed on {host.backend.hostname} for pid {pid}\n"
            f"STDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}"
        )
    return _key_values(result.stdout)


def xray_pid(host: Host) -> int:
    result = host.run("pgrep -f '/etc/xp2p/bin/[x]ray' | head -n1")
    if result.rc != 0 or not (result.stdout or "").strip():
        raise AssertionError(f"xray pid not found on {host.backend.hostname}")
    return int(result.stdout.strip())


def collect(host: Host, pid: int, peer_ip: str, count: int, interval: float) -> list[dict[str, int]]:
    samples = []
    for index in range(count):
        samples.append(process_sample(host, pid, peer_ip))
        if index + 1 < count:
            time.sleep(interval)
    return samples


def write_artifact(name: str, payload: dict) -> Path:
    root = Path(".logs/tests")
    root.mkdir(parents=True, exist_ok=True)
    stamp = time.strftime("%Y%m%d-%H%M%S")
    path = root / f"{name}-{stamp}.json"
    path.write_text(json.dumps(payload, indent=2, sort_keys=True), encoding="utf-8")
    return path


def _key_values(output: str | None) -> dict[str, int]:
    values = {}
    for line in (output or "").splitlines():
        key, separator, value = line.partition("=")
        if separator:
            values[key] = int(value.strip() or "0")
    return values
