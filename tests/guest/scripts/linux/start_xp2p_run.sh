#!/bin/bash
set -euo pipefail

if [ "$#" -lt 4 ]; then
  echo "Usage: start_xp2p_run.sh <role> <install_dir> <config_dir> <log_path> [extra...]" >&2
  exit 2
fi

ROLE=$1
INSTALL_DIR=$2
CONFIG_DIR=$3
LOG_PATH=$4
shift 4 || true

if [ "$ROLE" != "server" ] && [ "$ROLE" != "client" ]; then
  echo "Unsupported role: $ROLE" >&2
  exit 2
fi

mkdir -p "$INSTALL_DIR"
mkdir -p "$(dirname "$LOG_PATH")"

CMD=(/usr/bin/xp2p "$ROLE" run --path "$INSTALL_DIR" --config-dir "$CONFIG_DIR" --auto-install --xray-log-file "$LOG_PATH" --quiet)
if [ "$#" -gt 0 ]; then
  CMD+=("$@")
fi
nohup "${CMD[@]}" >/tmp/xp2p-${ROLE}-run.log 2>&1 &
PID=$!
sleep 1
if ! kill -0 "$PID" >/dev/null 2>&1; then
  echo "__XP2P_PID__="
  exit 3
fi

echo "__XP2P_PID__=$PID"
