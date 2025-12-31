#!/bin/sh
set -eu

iface="${1:-}"
if [ -z "$iface" ]; then
  echo "interface name is required" >&2
  exit 1
fi

ip_addr=$(ip -o -4 addr show dev "$iface" | awk '{print $4}' | cut -d/ -f1 | head -n1)
if [ -z "$ip_addr" ]; then
  echo "no IPv4 address found on interface $iface" >&2
  exit 1
fi

printf '%s\n' "$ip_addr"
