from __future__ import annotations

from dataclasses import dataclass
from concurrent.futures import ThreadPoolExecutor
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


def process_sample(host: Host, pid: int, peer_ip: str, runtime_metrics_file: str = "") -> dict[str, int]:
    script = r"""
pid=$1
peer=$2
metrics=$3
test -r "/proc/$pid/status" || { echo "process $pid disappeared" >&2; exit 20; }
kill -0 "$pid" 2>/dev/null || { echo "process $pid is not alive" >&2; exit 21; }
read rss threads < <(awk '/VmRSS:/ {rss=$2; seen_rss=1} /Threads:/ {threads=$2; seen_threads=1}
  END {if (!seen_rss || !seen_threads) exit 22; print rss, threads}' /proc/$pid/status) || exit $?
fd=$(find /proc/$pid/fd -maxdepth 1 -type l 2>/dev/null | wc -l)
links=$(find /proc/$pid/fd -maxdepth 1 -type l -printf '%l\n' 2>/dev/null)
socket_fd=$(printf '%s\n' "$links" | grep -c '^socket:' || true)
pipe_fd=$(printf '%s\n' "$links" | grep -c '^pipe:' || true)
anon_fd=$(printf '%s\n' "$links" | grep -c '^anon_inode:' || true)
tcp=$(ss -Htanp | awk -v marker="pid=$pid," -v peer="$peer" '
  index($0, marker) {
    total++
    state[$1]++
    remote=$5
    sub(/^\[/, "", remote); sub(/\](:[0-9*]+)?$/, "", remote); sub(/:[0-9*]+$/, "", remote)
    sub(/^::ffff:/, "", remote)
    peers[remote]++
    if (remote == peer) peer_total++
  }
  END {
    print "tcp_total=" total+0
    print "tcp_peer=" peer_total+0
    print "tcp_estab=" state["ESTAB"]+0
    for (name in state) if (name != "ESTAB") print "tcp_" tolower(name) "=" state[name]
    for (name in peers) {
      key=name
      gsub(/[^0-9A-Za-z]/, "_", key)
      print "tcp_peer_" key "=" peers[name]
    }
  }')
cgroup=$(awk -F: '$1 == "0" {print $3}' /proc/$pid/cgroup)
memory=-1
if [ -n "$cgroup" ] && [ -r "/sys/fs/cgroup${cgroup}/memory.current" ]; then
  memory=$(cat "/sys/fs/cgroup${cgroup}/memory.current")
else
  cgroup=$(awk -F: '$2 ~ /(^|,)memory(,|$)/ {print $3}' /proc/$pid/cgroup)
  if [ -n "$cgroup" ] && [ -r "/sys/fs/cgroup/memory${cgroup}/memory.usage_in_bytes" ]; then
    memory=$(cat "/sys/fs/cgroup/memory${cgroup}/memory.usage_in_bytes")
  fi
fi
test "$memory" -ge 0 || { echo "cgroup memory is unavailable for pid $pid" >&2; exit 26; }
printf 'rss_kib=%s\nthreads=%s\nfd=%s\nsocket_fd=%s\npipe_fd=%s\nanon_fd=%s\ncgroup_memory=%s\n' \
  "$rss" "$threads" "$fd" "$socket_fd" "$pipe_fd" "$anon_fd" "$memory"
printf '%s\n' "$tcp"
if [ -n "$metrics" ]; then
  test -r "$metrics" || { echo "runtime metrics are unavailable: $metrics" >&2; exit 23; }
  metrics_pid=$(awk -F= '$1 == "pid" {print $2}' "$metrics")
  test "$metrics_pid" = "$pid" || { echo "runtime metrics pid mismatch" >&2; exit 24; }
  sampled=$(awk -F= '$1 == "sample_unix_nano" {print $2}' "$metrics")
  now=$(date +%s%N)
  test -n "$sampled" && test $((now-sampled)) -lt 5000000000 ||
    { echo "runtime metrics are stale" >&2; exit 25; }
  awk -F= '$1 ~ /^(go_heap_alloc|go_heap_sys|go_goroutines|sample_unix_nano)$/ {print $1 "=" $2}' "$metrics"
fi
"""
    command = (
        "sudo -n bash -c "
        + shlex.quote(script)
        + f" -- {int(pid)} {shlex.quote(peer_ip)} {shlex.quote(runtime_metrics_file)}"
    )
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


def collect_parallel(
    owners: dict[str, tuple[Host, int, str, str]],
    samples: dict[str, list[dict[str, int]]],
    count: int,
    interval: float,
) -> None:
    for index in range(count):
        with ThreadPoolExecutor(max_workers=len(owners)) as executor:
            futures = {
                name: executor.submit(process_sample, host, pid, peer_ip, metrics_file)
                for name, (host, pid, peer_ip, metrics_file) in owners.items()
            }
            for name, future in futures.items():
                samples[name].append(future.result())
        if index + 1 < count:
            time.sleep(interval)


def host_peer_tcp(host: Host, peer_ip: str) -> int:
    script = r"""
peer=$1
ss -Htan state established | awk -v peer="$peer" '{
  remote=$5
  sub(/^\[/, "", remote); sub(/\](:[0-9*]+)?$/, "", remote); sub(/:[0-9*]+$/, "", remote)
  sub(/^::ffff:/, "", remote)
  if (remote == peer) count++
} END {print count+0}'
"""
    result = host.run("sudo -n bash -c " + shlex.quote(script) + f" -- {shlex.quote(peer_ip)}")
    if result.rc != 0:
        raise AssertionError(f"host TCP sampling failed on {host.backend.hostname}: {result.stderr}")
    return int((result.stdout or "").strip())


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
