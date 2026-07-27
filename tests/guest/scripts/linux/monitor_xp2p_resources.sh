#!/bin/sh

set -eu

interval_seconds="${INTERVAL_SECONDS:-300}"
sample_count="${SAMPLE_COUNT:-13}"
log_dir="${LOG_DIR:-/srv/xray-p2p/build/logs/linux}"
log_file="$log_dir/resource-hour-$(hostname)-$(date -u +%Y%m%dT%H%M%SZ).log"

mkdir -p "$log_dir"

sample=1
while [ "$sample" -le "$sample_count" ]; do
  {
    echo "===== sample=$sample/$sample_count time=$(date -u +%Y-%m-%dT%H:%M:%SZ) ====="
    uptime
    free -w 2>/dev/null || free
    echo "-- xp2p/xray processes --"

    pids="$(pgrep -x xp2p 2>/dev/null || true)
$(pgrep -x xray 2>/dev/null || true)"
    if [ -z "$(printf '%s' "$pids" | tr -d '[:space:]')" ]; then
      echo "no xp2p/xray processes"
    else
      for pid in $pids; do
        [ -r "/proc/$pid/status" ] || continue
        grep -E "^(Name|Pid|PPid|VmPeak|VmSize|VmHWM|VmRSS|Threads):" "/proc/$pid/status"
        fd_count="$(find "/proc/$pid/fd" -mindepth 1 -maxdepth 1 2>/dev/null | wc -l)"
        echo "FDCount: $fd_count"
        echo
      done
    fi

    echo "-- TCP states --"
    ss -tan 2>/dev/null |
      awk 'NR > 1 { count[$1]++ } END { for (state in count) print state, count[state] }' |
      sort
    echo
  } >>"$log_file"

  sample=$((sample + 1))
  [ "$sample" -le "$sample_count" ] && sleep "$interval_seconds"
done

echo "$log_file"
