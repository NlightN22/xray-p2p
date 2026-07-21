#!/bin/sh
set -eu

action="${1:-}"
mode="${2:-}"
root="/tmp/xp2p-3xui-snapshot-override"
body="$root/sub/xp2pfixture2811"
original="$root/original.txt"
pid_file="$root/server.pid"
url="http://127.0.0.1:2096/sub/xp2pfixture2811"

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

capture_snapshot() {
  mkdir -p "$root/sub"
  curl --fail --silent "$url" >"$original"
}

decode_snapshot() {
  if grep -q '://' "$original"; then
    cat "$original"
  else
    base64 -d "$original"
  fi
}

write_snapshot() {
  case "$mode" in
    malformed)
      printf '%s\n' 'malformed-snapshot' >"$body"
      ;;
    oversized)
      dd if=/dev/zero bs=1048576 count=5 2>/dev/null | tr '\000' 'x' >"$body"
      ;;
    required)
      decode_snapshot | sed 's/#/\&unsupported=required#/g' >"$body"
      ;;
    optional)
      decode_snapshot | sed 's/#/\&x-optional-provider-note=ignored#/g' >"$body"
      ;;
    *)
      echo "unsupported snapshot override mode" >&2
      exit 2
      ;;
  esac
}

case "$action" in
  capture)
    stop_server
    capture_snapshot
    cp "$original" /tmp/xp2p-3xui-original-subscription.txt
    ;;
  start)
    mkdir -p "$root/sub"
    cp /tmp/xp2p-3xui-original-subscription.txt "$original"
    write_snapshot
    nohup python3 -m http.server 2096 --bind 0.0.0.0 --directory "$root" >"$root/server.log" 2>&1 &
    echo "$!" >"$pid_file"
    attempt=0
    until curl --fail --silent "$url" >/dev/null; do
      attempt=$((attempt + 1))
      [ "$attempt" -lt 20 ] || { echo "snapshot override did not become ready" >&2; exit 1; }
      sleep 1
    done
    ;;
  stop)
    stop_server
    rm -f /tmp/xp2p-3xui-original-subscription.txt
    ;;
  *)
    echo "usage: snapshot-override.sh capture|start <mode>|stop" >&2
    exit 2
    ;;
esac
