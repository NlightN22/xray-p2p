#!/bin/sh
set -eu

if [ "$(id -u)" -eq 0 ]; then
  AS_ROOT=""
elif command -v sudo >/dev/null 2>&1; then
  AS_ROOT="sudo"
elif command -v doas >/dev/null 2>&1; then
  AS_ROOT="doas"
else
  echo "cleanup requires root privileges" >&2
  exit 1
fi

run_root() {
  if [ -z "$AS_ROOT" ]; then
    "$@"
  else
    "$AS_ROOT" "$@"
  fi
}

if command -v rc-service >/dev/null 2>&1; then
  run_root rc-service dnsmasq stop >/dev/null 2>&1 || true
fi

if command -v rc-update >/dev/null 2>&1; then
  run_root rc-update del dnsmasq default >/dev/null 2>&1 || true
fi

backup="$(ls -t /etc/dnsmasq.conf.bak.* 2>/dev/null | head -n 1 || true)"
if [ -n "$backup" ]; then
  run_root mv "$backup" /etc/dnsmasq.conf
else
  run_root rm -f /etc/dnsmasq.conf
fi

run_root rm -f /var/log/dnsmasq.log >/dev/null 2>&1 || true
