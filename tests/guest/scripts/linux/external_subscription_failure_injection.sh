#!/bin/sh
set -eu

action="${1:-}"
config_root="${XP2P_CONFIG_ROOT:-/etc/xp2p}"
desired="$config_root/xp2p-client.toml"
live="$config_root/.state/live/config-client"
lkg="$config_root/.state/subscriptions/fixture.json"

target_path() {
  case "$1" in
    desired) printf '%s\n' "$desired" ;;
    live) printf '%s\n' "$live" ;;
    lkg) printf '%s\n' "$lkg" ;;
    *) echo "unsupported persistence target" >&2; exit 2 ;;
  esac
}

case "$action" in
  protect)
    target="$(target_path "${2:-}")"
    mount --bind "$target" "$target"
    mount -o remount,bind,ro "$target"
    ;;
  unprotect)
    target="$(target_path "${2:-}")"
    mountpoint -q "$target" && umount "$target" || true
    ;;
  freeze-runtime)
    systemctl kill --kill-who=all --signal=STOP xp2p-client.service
    ;;
  unfreeze-runtime)
    systemctl kill --kill-who=all --signal=CONT xp2p-client.service 2>/dev/null || true
    ;;
  concurrent-refresh)
    source_host="${2:-10.62.10.13}"
    interface="$(ip route get "$source_host" | awk '{for (i=1; i<=NF; i++) if ($i == "dev") {print $(i+1); exit}}')"
    [ -n "$interface" ] || { echo "subscription route interface is missing" >&2; exit 1; }
    tc qdisc replace dev "$interface" root netem delay 1500ms
    trap 'tc qdisc del dev "$interface" root 2>/dev/null || true' EXIT
    xp2p client subscription refresh fixture --allow-http >/tmp/xp2p-concurrent-refresh.log 2>&1 &
    refresh_pid="$!"
    sleep 1
    printf '\n# concurrent-user-edit\n' >>"$desired"
    set +e
    wait "$refresh_pid"
    refresh_rc="$?"
    set -e
    grep -q 'changed concurrently' /tmp/xp2p-concurrent-refresh.log
    [ "$refresh_rc" -ne 0 ]
    ;;
  *)
    echo "usage: external_subscription_failure_injection.sh protect|unprotect <target>|freeze-runtime|unfreeze-runtime|concurrent-refresh [host]" >&2
    exit 2
    ;;
esac
