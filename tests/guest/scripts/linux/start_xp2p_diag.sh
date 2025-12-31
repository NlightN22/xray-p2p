#!/bin/sh
set -eu

listen="${1:-0.0.0.0:62022}"
proto="${2:-tcp}"
pid_file="${3:-/tmp/xp2p-diag.pid}"
log_file="${4:-/tmp/xp2p-diag.log}"

xp2p_bin="/srv/xray-p2p/build/linux-amd64/xp2p"
if [ ! -x "$xp2p_bin" ]; then
  echo "xp2p binary not found at $xp2p_bin" >&2
  exit 1
fi

port="${listen##*:}"
netstat_cmd="netstat -ltn"
if [ "$proto" = "udp" ]; then
  netstat_cmd="netstat -lun"
fi

nohup "$xp2p_bin" diag --listen "$listen" --proto "$proto" --quiet >"$log_file" 2>&1 &
pid=$!
echo "$pid" >"$pid_file"

timeout=20
while [ "$timeout" -gt 0 ]; do
  if $netstat_cmd 2>/dev/null | grep -q ":$port "; then
    exit 0
  fi
  timeout=$((timeout - 1))
  sleep 1
done

echo "diagnostics listener did not start on $listen" >&2
cat "$log_file" >&2 || true
exit 1
