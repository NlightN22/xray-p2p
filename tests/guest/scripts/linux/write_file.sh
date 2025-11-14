#!/bin/bash
set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "Usage: write_file.sh <path> <base64>" >&2
  exit 2
fi

target=$1
encoded=$2
mkdir -p "$(dirname "$target")"
printf '%s' "$encoded" | base64 -d >"$target"
