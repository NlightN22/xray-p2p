#!/bin/bash
set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "Usage: stop_process.sh <pid>" >&2
  exit 2
fi

PID=$1
if ! [[ "$PID" =~ ^[0-9]+$ ]]; then
  exit 0
fi

if ! kill -0 "$PID" >/dev/null 2>&1; then
  exit 0
fi

kill "$PID" >/dev/null 2>&1 || true

for _ in $(seq 1 20); do
  if ! kill -0 "$PID" >/dev/null 2>&1; then
    exit 0
  fi
  sleep 0.5
done

kill -9 "$PID" >/dev/null 2>&1 || true

exit 0
