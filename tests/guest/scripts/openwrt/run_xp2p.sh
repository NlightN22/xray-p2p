#!/bin/sh
set -eu

if ! command -v xp2p >/dev/null 2>&1; then
  echo "xp2p executable not found in PATH" >&2
  exit 1
fi

exec xp2p "$@"
