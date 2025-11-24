#!/bin/sh
set -eu

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

LOG_DIR=$(dirname "$LOG_PATH")
mkdir -p "$LOG_DIR"
: >"$LOG_PATH"
chmod 600 "$LOG_PATH"

EXTRA_ARGS="$*"
set -- client deploy --host "$REMOTE_HOST" --port "$DEPLOY_PORT" --user "$TROJAN_USER" --password "$TROJAN_PASSWORD"
if [ -n "$TROJAN_PORT" ]; then
  set -- "$@" --trojan-port "$TROJAN_PORT"
fi
if [ -n "$EXTRA_ARGS" ]; then
  # shellcheck disable=SC2086
  set -- "$@" $EXTRA_ARGS
fi

setsid /usr/bin/xp2p "$@" >"$LOG_PATH" 2>&1 &
PID=$!
sleep 1
if ! kill -0 "$PID" >/dev/null 2>&1; then
  echo "__XP2P_PID__="
  exit 3
fi

echo "__XP2P_PID__=$PID"
echo "__XP2P_LOG__=$LOG_PATH"
exit 0
