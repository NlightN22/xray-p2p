#!/bin/bash
set -euo pipefail

if [ "$#" -lt 5 ]; then
  echo "Usage: start_xp2p_run_with_env.sh <role> <install_dir> <config_dir> <allow_mismatch> <auto_install> [extra...]" >&2
  exit 2
fi

ROLE=$1
INSTALL_DIR=$2
CONFIG_DIR=$3
ALLOW_MISMATCH=$4
AUTO_INSTALL=$5
shift 5 || true

if [ "$ROLE" != "server" ] && [ "$ROLE" != "client" ]; then
  echo "Unsupported role: $ROLE" >&2
  exit 2
fi

CMD=(/usr/bin/xp2p "$ROLE" run --path "$INSTALL_DIR" --config-dir "$CONFIG_DIR" --quiet)
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
