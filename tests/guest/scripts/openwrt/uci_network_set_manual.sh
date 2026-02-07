#!/bin/sh
set -eu

name="${1:-}"
device="${2:-}"
proto="${3:-}"
ipaddr="${4:-}"

if [ -z "$name" ] || [ -z "$device" ] || [ -z "$proto" ] || [ -z "$ipaddr" ]; then
  echo "usage: $0 <name> <device> <proto> <ipaddr>" >&2
  exit 2
fi

uci -q delete "network.$name" >/dev/null 2>&1 || true
uci set "network.$name=interface"
uci set "network.$name.device=$device"
uci set "network.$name.proto=$proto"
uci add_list "network.$name.ipaddr=$ipaddr"
uci commit network
