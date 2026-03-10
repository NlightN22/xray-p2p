#!/bin/bash
set -euo pipefail

if [ "$#" -lt 3 ]; then
  echo "Usage: start_xp2p_server_deploy.sh <log_path> <listen_addr> <deploy_link> [extra...]" >&2
  exit 2
fi

LOG_PATH=$1
LISTEN_ADDR=$2
DEPLOY_LINK=$3
shift 3 || true

touch "$LOG_PATH"
chmod 600 "$LOG_PATH"

GLOBAL_ARGS=()
if [ -n "${XP2P_GLOBAL_ARGS:-}" ]; then
  read -r -a GLOBAL_ARGS <<<"$XP2P_GLOBAL_ARGS"
fi

CMD=(/usr/bin/xp2p "${GLOBAL_ARGS[@]}" server deploy --listen "$LISTEN_ADDR" --link "$DEPLOY_LINK")
if [ "$#" -gt 0 ]; then
  CMD+=("$@")
fi

nohup "${CMD[@]}" >"$LOG_PATH" 2>&1 &
PID=$!
sleep 1
if ! kill -0 "$PID" >/dev/null 2>&1; then
  echo "__XP2P_PID__="
  exit 3
fi

echo "__XP2P_PID__=$PID"
echo "__XP2P_LOG__=$LOG_PATH"
