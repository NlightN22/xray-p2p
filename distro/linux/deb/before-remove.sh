#!/bin/sh
set -eu

stop_service() {
  service_name="$1"
  if command -v systemctl >/dev/null 2>&1; then
    systemctl stop "$service_name" >/dev/null 2>&1 || true
    systemctl disable "$service_name" >/dev/null 2>&1 || true
  fi
}

rollback_full_tunnel() {
  if ! command -v xp2p >/dev/null 2>&1; then
    return 0
  fi

  mode_output="$(xp2p client mode 2>/dev/null || true)"
  echo "$mode_output" | grep -qi "mode: tun" || return 0
  echo "$mode_output" | grep -qi "tun_mode: full" || return 0

  xp2p client mode tun split -q >/dev/null 2>&1 || true
}

rollback_full_tunnel
stop_service xp2p-client.service
stop_service xp2p-server.service

exit 0
