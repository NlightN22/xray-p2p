#!/bin/sh
set -eu

ip_addr=$(ip -o -4 addr show scope global | awk '{print $4}' | cut -d/ -f1 | head -n1)
if [ -z "$ip_addr" ]; then
  echo "no IPv4 address found" >&2
  exit 3
fi

printf '%s\n' "$ip_addr"
