#!/bin/sh
set -eu

name="${1:-}"
if [ -z "$name" ]; then
  echo "interface name required" >&2
  exit 2
fi

uci -q show "network.$name" || true
