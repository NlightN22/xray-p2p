#!/bin/bash
set -euo pipefail

if [ "$#" -lt 1 ]; then
  echo "Usage: bundle_backup_check.sh <root> [marker|--expect-none]" >&2
  exit 2
fi

root=$1
mode=${2:-""}

shopt -s nullglob
backups=( "${root}.bak-"* )

if [ "$mode" = "--expect-none" ]; then
  if [ "${#backups[@]}" -ne 0 ]; then
    echo "Unexpected backup directories for $root" >&2
    exit 1
  fi
  exit 0
fi

if [ "${#backups[@]}" -eq 0 ]; then
  echo "No backup directories for $root" >&2
  exit 1
fi

marker=$mode
if [ -n "$marker" ]; then
  if [ ! -f "${backups[0]}/$marker" ]; then
    echo "Marker $marker not found in ${backups[0]}" >&2
    exit 1
  fi
fi
