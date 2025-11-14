#!/bin/bash
set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "Usage: remove_path.sh <path>" >&2
  exit 2
fi

target=$1
if [ -e "$target" ]; then
  rm -rf "$target"
fi
