#!/bin/bash
set -euo pipefail

if [ "$#" -lt 2 ]; then
  echo "Usage: bundle_export_default.sh <cwd> <role> [config-root|-]" >&2
  exit 2
fi

cwd=$1
role=$2
config_root=${3:-"-"}
pattern="xp2p-${role}-backup-*.tar.gz"

if [ ! -d "$cwd" ]; then
  echo "Output directory is missing: $cwd" >&2
  exit 1
fi
cd "$cwd"

shopt -s nullglob
before=( $pattern )

if [ "$config_root" = "-" ]; then
  /usr/bin/xp2p "$role" export
else
  XP2P_CONFIG_ROOT="$config_root" /usr/bin/xp2p "$role" export
fi

after=( $pattern )
if [ "${#after[@]}" -le "${#before[@]}" ]; then
  echo "Expected new archive in $cwd" >&2
  exit 1
fi

latest=$(ls -1t $pattern | head -n1)
echo "__XP2P_ARCHIVE__=$cwd/$latest"
