#!/bin/sh
set -eu

INSTALL_URL="${1:-https://nlightn22.github.io/xray-p2p/install-xp2p.sh}"

KEY_ID="a371ae624079a206"
KEY_PATH="/etc/opkg/keys/${KEY_ID}"
FEED_CONF_MAIN="/etc/opkg/customfeeds.conf"
FEED_DIR="/etc/opkg/customfeeds.d"
FEED_FALLBACK="${FEED_DIR}/99-xp2p.conf"

log() {
  echo "$*"
}

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

backup_file() {
  src="$1"
  dst="$2"
  if [ -e "$src" ]; then
    cat "$src" >"$dst"
    echo "1"
  else
    echo "0"
  fi
}

restore_file() {
  src="$1"
  dst="$2"
  existed="$3"
  if [ "$existed" = "1" ]; then
    cat "$src" >"$dst"
  else
    rm -f "$dst"
  fi
}

if [ "$(id -u)" != "0" ]; then
  fail "expected to run as root"
fi

tmp_dir="$(mktemp -d "/tmp/xp2p-install-script.XXXXXX")"
cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT

main_backup="$tmp_dir/customfeeds.conf.bak"
fallback_backup="$tmp_dir/99-xp2p.conf.bak"
key_backup="$tmp_dir/key.bak"

main_existed="$(backup_file "$FEED_CONF_MAIN" "$main_backup")"
fallback_existed="$(backup_file "$FEED_FALLBACK" "$fallback_backup")"
key_existed="$(backup_file "$KEY_PATH" "$key_backup")"

log "Running install command from docs"
out="$(wget -qO- "$INSTALL_URL" 2>&1 | sh 2>&1)" || {
  echo "$out"
  fail "install command failed"
}

log "Verifying installation"
command -v xp2p >/dev/null 2>&1 || fail "xp2p binary not found after install"
xp2p --version >/dev/null 2>&1 || fail "xp2p --version failed after install"
test -s "$KEY_PATH" || fail "opkg signing key not found at $KEY_PATH"
opkg list-installed 2>/dev/null | grep -q "^xp2p - " || fail "xp2p package not installed"

if ! grep -q "^src/gz xp2p " "$FEED_CONF_MAIN" 2>/dev/null && ! grep -q "^src/gz xp2p " "$FEED_FALLBACK" 2>/dev/null; then
  fail "xp2p feed entry not found in opkg config"
fi

log "Cleaning up"
/etc/init.d/xp2p-client stop >/dev/null 2>&1 || true
/etc/init.d/xp2p-server stop >/dev/null 2>&1 || true
opkg remove xp2p >/dev/null 2>&1 || true

restore_file "$main_backup" "$FEED_CONF_MAIN" "$main_existed"
restore_file "$fallback_backup" "$FEED_FALLBACK" "$fallback_existed"
restore_file "$key_backup" "$KEY_PATH" "$key_existed"

log "OK"

