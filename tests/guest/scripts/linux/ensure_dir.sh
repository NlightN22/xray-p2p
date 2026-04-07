#!/bin/sh
set -e

if [ "$#" -lt 1 ]; then
  echo "Usage: ensure_dir.sh <path>" >&2
  exit 2
fi

target=$1
if [ ! -d "$target" ]; then
  echo "Expected directory does not exist: $target" >&2
  exit 3
fi
