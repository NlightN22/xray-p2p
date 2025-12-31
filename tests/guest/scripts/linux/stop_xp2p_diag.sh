#!/bin/sh
set -eu

pid_file="${1:-/tmp/xp2p-diag.pid}"
if [ ! -f "$pid_file" ]; then
  exit 0
fi

pid=$(cat "$pid_file" 2>/dev/null || true)
if [ -n "$pid" ]; then
  kill "$pid" >/dev/null 2>&1 || true
  wait "$pid" >/dev/null 2>&1 || true
fi
rm -f "$pid_file"
