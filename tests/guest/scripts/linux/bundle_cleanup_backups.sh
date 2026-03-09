#!/bin/bash
set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "Usage: bundle_cleanup_backups.sh <root>" >&2
  exit 2
fi

root=$1

shopt -s nullglob
backups=( "${root}.bak-"* )
for path in "${backups[@]}"; do
  rm -rf "$path"
done
