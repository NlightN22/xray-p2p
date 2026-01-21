#!/bin/sh
set -eu

all_addrs=$(ip -o -4 addr show scope global | awk '{print $4}' | cut -d/ -f1)
if [ -z "$all_addrs" ]; then
  echo "no IPv4 address found" >&2
  exit 3
fi

chosen=$(printf '%s\n' "$all_addrs" | grep -v '^10\.0\.2\.' | head -n1 || true)
if [ -z "$chosen" ]; then
  chosen=$(printf '%s\n' "$all_addrs" | head -n1)
fi

printf '%s\n' "$chosen"
