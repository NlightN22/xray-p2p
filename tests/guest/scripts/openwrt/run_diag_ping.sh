#!/bin/sh
set -eu

listen="${1:-}"
proto="${2:-tcp}"

if [ -z "$listen" ]; then
  echo "listen address is required" >&2
  exit 1
fi

if ! command -v xp2p >/dev/null 2>&1; then
  echo "xp2p executable not found in PATH" >&2
  exit 1
fi

port="${listen##*:}"
diag_log="/tmp/xp2p-diag.log"
ping_log="/tmp/xp2p-diag-ping.log"
netstat_cmd="netstat -ltn"
if [ "$proto" = "udp" ]; then
  netstat_cmd="netstat -lun"
fi

xp2p diag --listen "$listen" --proto "$proto" --quiet >"$diag_log" 2>&1 &
pid=$!

cleanup() {
  kill "$pid" >/dev/null 2>&1 || true
  wait "$pid" >/dev/null 2>&1 || true
}
trap cleanup EXIT

timeout=20
while [ "$timeout" -gt 0 ]; do
  if $netstat_cmd 2>/dev/null | grep -q ":$port "; then
    break
  fi
  timeout=$((timeout - 1))
  sleep 1
done

if [ "$timeout" -le 0 ]; then
  echo "diagnostics listener did not start on $listen" >&2
  cat "$diag_log" >&2 || true
  exit 1
fi

if ! xp2p ping 127.0.0.1 --proto "$proto" --port "$port" --count 1 >"$ping_log" 2>&1; then
  echo "xp2p ping failed" >&2
  cat "$ping_log" >&2 || true
  exit 1
fi
