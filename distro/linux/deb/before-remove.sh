#!/bin/sh
set -eu

stop_service() {
  service_name="$1"
  if command -v systemctl >/dev/null 2>&1; then
    systemctl stop "$service_name" >/dev/null 2>&1 || true
    systemctl disable "$service_name" >/dev/null 2>&1 || true
  fi
}

stop_service xp2p-client.service
stop_service xp2p-server.service

exit 0
