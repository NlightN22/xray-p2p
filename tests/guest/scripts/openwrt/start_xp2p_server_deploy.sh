#!/bin/sh
set -eu

if [ "$#" -lt 3 ]; then
  echo "Usage: start_xp2p_server_deploy.sh <log_path> <listen_addr> <deploy_link> [ENV=VAL ...] [-- extra...]" >&2
  exit 2
fi

LOG_PATH=$1
LISTEN_ADDR=$2
DEPLOY_LINK=$3
shift 3 || true

: >"$LOG_PATH"

ENV_ARGS=""
EXTRA_ARGS=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --)
      shift
      EXTRA_ARGS="$*"
      break
      ;;
    *=*)
      if [ -n "$ENV_ARGS" ]; then
        ENV_ARGS="$ENV_ARGS $1"
      else
        ENV_ARGS="$1"
      fi
      shift
      ;;
    *)
      EXTRA_ARGS="$*"
      break
      ;;
  esac
done

set -- server deploy --listen "$LISTEN_ADDR" --link "$DEPLOY_LINK"

if [ -n "$EXTRA_ARGS" ]; then
  # shellcheck disable=SC2086
  set -- "$@" $EXTRA_ARGS
fi

if [ -n "$ENV_ARGS" ]; then
  # shellcheck disable=SC2086
  setsid env $ENV_ARGS /usr/bin/xp2p "$@" >"$LOG_PATH" 2>&1 &
else
  setsid /usr/bin/xp2p "$@" >"$LOG_PATH" 2>&1 &
fi
PID=$!
sleep 1
if ! kill -0 "$PID" >/dev/null 2>&1; then
  echo "__XP2P_PID__="
  exit 3
fi

echo "__XP2P_PID__=$PID"
echo "__XP2P_LOG__=$LOG_PATH"
exit 0
