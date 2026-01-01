#!/bin/sh
set -eu

if [ "$#" -lt 1 ]; then
  echo "Usage: nslookup.sh <hostname> [server]" >&2
  exit 2
fi

name="$1"
server="${2:-}"
timeout_cmd=""

if command -v timeout >/dev/null 2>&1; then
  timeout_cmd="timeout ${NSLOOKUP_TIMEOUT:-15}"
fi

if [ -n "$server" ]; then
  if [ -n "$timeout_cmd" ]; then
    exec $timeout_cmd nslookup "$name" "$server"
  fi
  exec nslookup "$name" "$server"
fi

if [ -n "$timeout_cmd" ]; then
  exec $timeout_cmd nslookup "$name"
fi
exec nslookup "$name"
