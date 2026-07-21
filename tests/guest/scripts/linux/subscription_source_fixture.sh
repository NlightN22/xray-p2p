#!/bin/sh
set -eu

action="${1:-}"
root="/tmp/xp2p-subscription-source"
body="$root/subscription.txt"
pid_file="$root/server.pid"

write_mode() {
  mode="$1"
  mkdir -p "$root"
  case "$mode" in
    valid)
      printf '%s\n' 'trojan://fixture-negative-secret@127.0.0.1:443?security=tls&type=tcp&sni=localhost#Fixture' >"$body"
      ;;
    rotated)
      printf '%s\n' 'trojan://fixture-rotated-secret@127.0.0.1:443?security=tls&type=tcp&sni=localhost#Fixture' >"$body"
      ;;
    malformed)
      printf '%s\n' 'malformed-snapshot' >"$body"
      ;;
    unsupported)
      printf '%s\n' 'ftp://fixture-negative-secret@127.0.0.1:443#Fixture' >"$body"
      ;;
    oversized)
      dd if=/dev/zero bs=1048576 count=5 2>/dev/null | tr '\000' 'x' >"$body"
      ;;
    *)
      echo "unsupported subscription fixture mode" >&2
      exit 2
      ;;
  esac
}

stop_server() {
  if [ -f "$pid_file" ]; then
    pid="$(cat "$pid_file")"
    case "$pid" in
      *[!0-9]*|'') ;;
      *) kill "$pid" 2>/dev/null || true ;;
    esac
  fi
  rm -rf "$root"
}

case "$action" in
  start)
    stop_server
    write_mode valid
    nohup python3 -m http.server 18096 --bind 127.0.0.1 --directory "$root" >"$root/server.log" 2>&1 &
    echo "$!" >"$pid_file"
    attempt=0
    until curl --fail --silent http://127.0.0.1:18096/subscription.txt >/dev/null; do
      attempt=$((attempt + 1))
      [ "$attempt" -lt 20 ] || { echo "subscription fixture did not become ready" >&2; exit 1; }
      sleep 1
    done
    ;;
  set)
    write_mode "${2:-}"
    ;;
  stop)
    stop_server
    ;;
  *)
    echo "usage: subscription_source_fixture.sh start|set <mode>|stop" >&2
    exit 2
    ;;
esac
