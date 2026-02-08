#!/bin/sh
set -eu

name="${1:-}"
addr="${2:-}"
timeout="${3:-10}"

if [ -z "$name" ] || [ -z "$addr" ]; then
  echo "usage: $0 <name> <addr> [timeout_seconds]" >&2
  exit 2
fi

end=$(( $(date +%s) + timeout ))
while [ "$(date +%s)" -le "$end" ]; do
  if ip -4 addr show dev "$name" 2>/dev/null | grep -Fq "$addr"; then
    exit 0
  fi
  sleep 1
done

echo "missing tun address $addr on $name" >&2
exit 1
