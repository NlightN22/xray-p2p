#!/bin/bash
set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "Usage: path_exists.sh <path>" >&2
  exit 2
fi

target=$1
if [ -e "$target" ]; then
  exit 0
fi

exit 3
