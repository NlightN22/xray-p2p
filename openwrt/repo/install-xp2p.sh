#!/bin/sh
# OpenWrt helper to add the xp2p feed, import the signing key, and install xp2p.

set -eu

BASE_URL="https://nlightn22.github.io/xray-p2p"
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

ensure_command() {
    command -v "$1" >/dev/null 2>&1 || fail "required command '$1' not found"
}

ensure_command wget
ensure_command opkg

if [ "$(id -u)" != "0" ]; then
    fail "this script must be run as root"
fi

# Detect OpenWrt release
release=""
if [ -f /etc/openwrt_release ]; then
    # shellcheck disable=SC1091
    . /etc/openwrt_release
    release="${DISTRIB_RELEASE:-}"
fi
if [ -z "$release" ] && [ -f /etc/os-release ]; then
    # shellcheck disable=SC1091
    . /etc/os-release
    release="${VERSION_ID:-}"
fi
if [ -z "$release" ]; then
    fail "unable to determine OpenWrt release version"
fi

# Detect primary architecture (highest priority from opkg print-architecture)
arch="$(opkg print-architecture 2>/dev/null | awk 'NF==3 {print $2, $3}' | sort -k2 -nr | head -n1 | awk '{print $1}')"
if [ -z "$arch" ]; then
    fail "unable to determine device architecture via 'opkg print-architecture'"
fi

feed_url="${BASE_URL}/${release}/${arch}"
log "Detected release=${release}, arch=${arch}"

# Import signing key
log "Installing feed signing key"
mkdir -p /etc/opkg/keys
tmp_key="$(mktemp "/tmp/xp2p-key.XXXXXX")"
if ! wget -q -O "$tmp_key" "${BASE_URL}/${KEY_ID}.pub"; then
    rm -f "$tmp_key"
    fail "failed to download signing key from ${BASE_URL}/${KEY_ID}.pub"
fi
mv "$tmp_key" "$KEY_PATH"
chmod 0644 "$KEY_PATH"

# Configure feed
log "Configuring feed at ${feed_url}"
if [ -w "$FEED_CONF_MAIN" ] || [ ! -e "$FEED_CONF_MAIN" ]; then
    FEED_FILE="$FEED_CONF_MAIN"
    touch "$FEED_FILE"
else
    mkdir -p "$FEED_DIR"
    FEED_FILE="$FEED_FALLBACK"
fi
if [ -e "$FEED_FILE" ]; then
    if command -v sed >/dev/null 2>&1; then
        sed -i '/^src\/gz xp2p /d' "$FEED_FILE"
    else
        tmp_feed="$(mktemp "/tmp/xp2p-feed.XXXXXX")"
        grep -v '^src/gz xp2p ' "$FEED_FILE" >"$tmp_feed" || true
        mv "$tmp_feed" "$FEED_FILE"
    fi
fi
printf 'src/gz xp2p %s\n' "$feed_url" >>"$FEED_FILE"

# Refresh and install
log "Running opkg update"
if ! opkg update; then
    fail "opkg update failed; verify network connectivity and feed URL $feed_url"
fi

log "Installing xp2p package"
if ! opkg install xp2p; then
    fail "opkg install xp2p failed"
fi

log "xp2p installation completed successfully"
