#!/bin/sh
set -eu

STATE_ROOT=${XP2P_PACKAGE_STATE_ROOT:-/var/lib/xp2p}
STATE_FILE="$STATE_ROOT/deb-upgrade-services"

restore_service_state() {
  while read -r service_name enabled active; do
    [ -n "$service_name" ] || continue
    if [ "$enabled" = "1" ]; then
      systemctl enable "$service_name" >/dev/null 2>&1 || true
    else
      systemctl disable "$service_name" >/dev/null 2>&1 || true
    fi
    if [ "$active" = "1" ]; then
      systemctl start "$service_name" >/dev/null 2>&1 || true
    else
      systemctl stop "$service_name" >/dev/null 2>&1 || true
    fi
  done <"$STATE_FILE"
  rm -f "$STATE_FILE"
}

if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload >/dev/null 2>&1 || true
  if [ -f "$STATE_FILE" ]; then
    restore_service_state
  else
    systemctl enable xp2p-client xp2p-server >/dev/null 2>&1 || true
  fi
fi

exit 0
