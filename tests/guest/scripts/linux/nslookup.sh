#!/bin/sh
set -eu

if [ "$#" -lt 1 ]; then
  echo "Usage: nslookup.sh <hostname> [server]" >&2
  exit 2
fi

name="$1"
server="${2:-}"

if [ -n "$server" ]; then
  exec nslookup "$name" "$server"
fi

exec nslookup "$name"
