#!/bin/bash
set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "Usage: read_file.sh <path>" >&2
  exit 2
fi

target=$1
if [ ! -f "$target" ]; then
  exit 3
fi

cat "$target"
