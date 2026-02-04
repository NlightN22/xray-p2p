#!/bin/bash
set -euo pipefail

if [ "$#" -lt 6 ]; then
  echo "Usage: start_xp2p_run_with_env.sh <role> <install_dir> <config_dir> <log_path> <allow_mismatch> <auto_install> [extra...]" >&2
  exit 2
fi

ROLE=$1
INSTALL_DIR=$2
CONFIG_DIR=$3
LOG_PATH=$4
ALLOW_MISMATCH=$5
AUTO_INSTALL=$6
shift 6 || true

if [ "$ROLE" != "server" ] && [ "$ROLE" != "client" ]; then
  echo "Unsupported role: $ROLE" >&2
  exit 2
fi

mkdir -p "$INSTALL_DIR"
mkdir -p "$(dirname "$LOG_PATH")"

CMD=(/usr/bin/xp2p "$ROLE" run --path "$INSTALL_DIR" --config-dir "$CONFIG_DIR" --xray-log-file "$LOG_PATH" --quiet)
if [ "$AUTO_INSTALL" = "1" ]; then
  CMD+=(--auto-install)
else
  CMD+=(--auto-install=false)
fi
if [ "$#" -gt 0 ]; then
  CMD+=("$@")
fi

RUN_LOG="/tmp/xp2p-${ROLE}-run.log"
rm -f "$RUN_LOG"

if [ "$ALLOW_MISMATCH" = "1" ]; then
  env XP2P_XRAY_ALLOW_MISMATCH=1 nohup "${CMD[@]}" >"$RUN_LOG" 2>&1 &
else
  nohup "${CMD[@]}" >"$RUN_LOG" 2>&1 &
fi
PID=$!
sleep 1
if ! kill -0 "$PID" >/dev/null 2>&1; then
  echo "__XP2P_PID__="
  exit 3
fi

echo "__XP2P_PID__=$PID"
