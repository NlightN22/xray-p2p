#!/bin/sh
set -eu

name="${1:-}"
if [ -z "$name" ]; then
  echo "interface name required" >&2
  exit 2
fi

uci -q delete "network.$name" >/dev/null 2>&1 || true
uci commit network
