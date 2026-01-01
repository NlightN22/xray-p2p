#!/bin/sh
set -eu

cidr="${1:-}"
gateway="${2:-}"
iface="${3:-eth1}"

if [ -z "$cidr" ] || [ -z "$gateway" ]; then
  echo "usage: ensure_route.sh <cidr> <gateway> [iface]" >&2
  exit 1
fi

cmd="ip route replace $cidr via $gateway dev $iface"
if [ "$(id -u)" -ne 0 ] && command -v sudo >/dev/null 2>&1; then
  cmd="sudo $cmd"
fi
sh -c "$cmd"
