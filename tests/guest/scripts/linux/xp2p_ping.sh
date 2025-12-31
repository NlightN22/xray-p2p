#!/bin/sh
set -eu

target="${1:-}"
shift || true

if [ -z "$target" ]; then
  echo "target host is required" >&2
  exit 1
fi

xp2p_bin="/srv/xray-p2p/build/linux-amd64/xp2p"
if [ ! -x "$xp2p_bin" ]; then
  echo "xp2p binary not found at $xp2p_bin" >&2
  exit 1
fi

exec "$xp2p_bin" ping "$target" "$@"
