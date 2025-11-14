#!/bin/bash
set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "Usage: file_sha256.sh <path>" >&2
  exit 2
fi

target=$1
if [ ! -f "$target" ]; then
  exit 3
fi

sha256sum "$target" | awk '{print $1}'
