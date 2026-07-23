#!/bin/sh
set -eu

STATE_ROOT=${XP2P_PACKAGE_STATE_ROOT:-/var/lib/xp2p}
STATE_FILE="$STATE_ROOT/deb-upgrade-services"

capture_service_state() {
  install -d -m 0755 "$STATE_ROOT"
  state_tmp="$STATE_FILE.tmp"
  {
    for service_name in xp2p-client.service xp2p-server.service; do
      enabled=0
      active=0
      systemctl is-enabled --quiet "$service_name" && enabled=1
      systemctl is-active --quiet "$service_name" && active=1
      printf '%s %s %s\n' "$service_name" "$enabled" "$active"
    done
  } >"$state_tmp"
  chmod 0600 "$state_tmp"
  mv -f "$state_tmp" "$STATE_FILE"
}

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

if [ "${1:-}" = "upgrade" ] && command -v systemctl >/dev/null 2>&1; then
  capture_service_state
else
  rollback_full_tunnel
fi
stop_service xp2p-client.service
stop_service xp2p-server.service

exit 0
