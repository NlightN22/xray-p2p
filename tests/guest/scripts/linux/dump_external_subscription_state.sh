#!/bin/sh
set -eu

config_root="${XP2P_CONFIG_ROOT:-/etc/xp2p}"
state_root="$config_root/.state"
log_root="${XP2P_LOG_ROOT:-/var/log/xp2p}"

sanitize() {
  sed -E \
    -e 's#https?://[^[:space:]]+#<redacted-url>#g' \
    -e 's#(trojan|vless)://[^@[:space:]]+@#\1://<redacted>@#g' \
    -e 's#[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}#<redacted-uuid>#g' \
    -e 's#(password|credential|secret)([=:][[:space:]]*)[^,[:space:]]+#\1\2<redacted>#Ig'
}

summarize_file() {
  label="$1"
  path="$2"
  echo "--- $label ---"
  if [ ! -f "$path" ]; then
    echo "missing"
    return
  fi
  wc -c "$path"
  sha256sum "$path"
}

echo "--- state tree ---"
find "$state_root" -maxdepth 5 -printf '%y %p\n' 2>/dev/null | sanitize || true

echo "--- subscription metadata ---"
xp2p client subscription status 2>&1 | sanitize || true

summarize_file "Desired" "$config_root/xp2p-client.toml"
grep -E '^[[:space:]]*(\[\[client\.(subscriptions|endpoints)\]\]|id|adapter|compatibility_version|profile|protocol|transport|security|hostname|port|tag)[[:space:]]*=' \
  "$config_root/xp2p-client.toml" 2>/dev/null | sanitize || true

live="$state_root/live/config-client/xray.json"
summarize_file "Live" "$live"
grep -oE '"(protocol|network|security)"[[:space:]]*:[[:space:]]*"[^"]*"' "$live" 2>/dev/null | sanitize || true

echo "--- subscription LKG files ---"
for path in "$state_root"/subscriptions/*.json; do
  [ -f "$path" ] || continue
  summarize_file "LKG" "$path"
  grep -oE '"(adapter|revision|protocol|transport|security)"[[:space:]]*:[[:space:]]*"[^"]*"' "$path" | sanitize || true
done

for marker in "$state_root/apply.request" "$state_root/apply.error"; do
  summarize_file "$(basename "$marker")" "$marker"
  grep -oE '"(id|role|request_id|created_at|updated_at)"[[:space:]]*:[[:space:]]*"[^"]*"' "$marker" 2>/dev/null | sanitize || true
done

echo "--- sanitized XP2P and Xray logs ---"
for path in "$log_root"/*.log "$log_root"/*/*.log; do
  [ -f "$path" ] || continue
  echo "--- log $(basename "$path") ---"
  tail -n 120 "$path" | sanitize
done
journalctl --no-pager -n 120 -u xp2p-client.service 2>/dev/null | sanitize || true
