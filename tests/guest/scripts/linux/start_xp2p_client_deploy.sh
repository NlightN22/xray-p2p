#!/bin/bash
set -euo pipefail

if [ "$#" -lt 6 ]; then
  echo "Usage: start_xp2p_client_deploy.sh <log_path> <remote_host> <deploy_port> <trojan_user> <trojan_password> <trojan_port> [extra...]" >&2
  exit 2
fi

LOG_PATH=$1
REMOTE_HOST=$2
DEPLOY_PORT=$3
TROJAN_USER=$4
TROJAN_PASSWORD=$5
TROJAN_PORT=$6
shift 6 || true

mkdir -p "$(dirname "$LOG_PATH")"
touch "$LOG_PATH"
chmod 600 "$LOG_PATH"

GLOBAL_ARGS=()
if [ -n "${XP2P_GLOBAL_ARGS:-}" ]; then
  read -r -a GLOBAL_ARGS <<<"$XP2P_GLOBAL_ARGS"
fi

CMD=(/usr/bin/xp2p "${GLOBAL_ARGS[@]}" client deploy --host "$REMOTE_HOST" --port "$DEPLOY_PORT" --user "$TROJAN_USER" --password "$TROJAN_PASSWORD")
if [ -n "$TROJAN_PORT" ]; then
  CMD+=("--trojan-port" "$TROJAN_PORT")
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
