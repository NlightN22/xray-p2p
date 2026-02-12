#!/bin/sh
set -e

if [ "$#" -lt 1 ]; then
  echo "Usage: ensure_dir.sh <path> [mode]" >&2
  exit 2
fi

target=$1
mode=${2:-}

mkdir -p "$target"
if [ -n "$mode" ]; then
  chmod "$mode" "$target"
fi
