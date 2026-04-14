#!/bin/bash
set -euo pipefail

if [ "$#" -lt 2 ]; then
  echo "Usage: start_xp2p_server_deploy.sh <log_path> [listen_addr] <deploy_link> [extra...]" >&2
  exit 2
fi

LOG_PATH=$1
if echo "$2" | grep -q '^trojan://'; then
  LISTEN_ADDR=""
  DEPLOY_LINK=$2
  shift 2 || true
else
  LISTEN_ADDR=$2
  DEPLOY_LINK=$3
  if [ -z "$DEPLOY_LINK" ]; then
    echo "Usage: start_xp2p_server_deploy.sh <log_path> [listen_addr] <deploy_link> [extra...]" >&2
    exit 2
  fi
  shift 3 || true
fi

touch "$LOG_PATH"

GLOBAL_ARGS=()
if [ -n "${XP2P_GLOBAL_ARGS:-}" ]; then
  read -r -a GLOBAL_ARGS <<<"$XP2P_GLOBAL_ARGS"
fi

CMD=(/usr/bin/xp2p "${GLOBAL_ARGS[@]}" server deploy --link "$DEPLOY_LINK")
if [ -n "$LISTEN_ADDR" ]; then
  CMD+=("--listen" "$LISTEN_ADDR")
fi
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
