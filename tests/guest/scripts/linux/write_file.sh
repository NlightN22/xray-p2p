#!/bin/bash
set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "Usage: write_file.sh <path> <base64>" >&2
  exit 2
fi

target=$1
encoded=$2
target_dir=$(dirname "$target")
if [ ! -d "$target_dir" ]; then
  echo "Destination directory does not exist: $target_dir" >&2
  exit 3
fi
printf '%s' "$encoded" | base64 -d >"$target"
