#!/bin/sh

set -eu

DEFAULT_PORT="48044"
DEFAULT_ZONE="lan"
LAN_SECTION="xray_transparent_lan"
LOCAL_SECTION="xray_transparent_local"

log() {
    printf '%s\n' "$*"
}

die() {
    printf 'Error: %s\n' "$*" >&2
    exit 1
}

usage() {
    cat <<'USAGE'
Usage: xray_redirect.sh [SUBNET] [PORT] [ZONE]

SUBNET  Destination subnet to divert (CIDR notation, e.g. 10.0.101.0/24).
PORT    Local XRAY dokodemo-door port to forward to (default 48044).
ZONE    OpenWrt firewall zone whose traffic should be intercepted (default lan).

The script ensures two firewall sections:
  * Redirect traffic from the specified zone towards SUBNET into the local XRAY port.
  * Redirect router-originated traffic to SUBNET as well.

Existing sections named 'xray_transparent_lan' and 'xray_transparent_local' are
updated in-place; other options are preserved.
USAGE
}

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
    usage
    exit 0
fi

SUBNET="${1:-}"
PORT="${2:-$DEFAULT_PORT}"
ZONE="${3:-$DEFAULT_ZONE}"

if [ -z "$SUBNET" ]; then
    if [ -t 0 ]; then
        printf 'Enter destination subnet (CIDR, e.g. 10.0.101.0/24): '
        IFS= read -r SUBNET
    elif [ -r /dev/tty ]; then
        printf 'Enter destination subnet (CIDR, e.g. 10.0.101.0/24): '
        IFS= read -r SUBNET </dev/tty
    else
        die "Subnet argument is required"
    fi
fi

if [ -z "$SUBNET" ]; then
    die "Subnet cannot be empty"
fi

# very light CIDR validation
case "$SUBNET" in
    *'/'*) ;;
    *) die "Subnet must be in CIDR notation (example: 10.0.101.0/24)" ;;
esac

case "$PORT" in
    ''|*[!0-9]*) die "Port must be numeric" ;;
    *)
        if [ "$PORT" -lt 1 ] || [ "$PORT" -gt 65535 ]; then
            die "Port must be between 1 and 65535"
        fi
        ;;
esac

if ! command -v uci >/dev/null 2>&1; then
    die "uci command not available (is this OpenWrt?)"
fi

ensure_section() {
    local section="$1"
    local type="$2"
    if uci -q get "firewall.$section" >/dev/null 2>&1; then
        uci set "firewall.$section=$type"
    else
        local newsec
        newsec=$(uci add firewall "$type")
        uci rename "firewall.$newsec=$section"
    fi
}

ensure_redirect_lan() {
    ensure_section "$LAN_SECTION" redirect
    uci set "firewall.$LAN_SECTION.name=XRAY transparent proxy (LAN)"
    uci set "firewall.$LAN_SECTION.enabled=1"
    uci set "firewall.$LAN_SECTION.family=ipv4"
    uci set "firewall.$LAN_SECTION.src=$ZONE"
    uci set "firewall.$LAN_SECTION.proto=tcp"
    uci set "firewall.$LAN_SECTION.target=redirect"
    uci set "firewall.$LAN_SECTION.dest_port=$PORT"
    uci set "firewall.$LAN_SECTION.dest_ip=$SUBNET"
    # remove optional matches that could interfere with full redirect
    uci -q delete "firewall.$LAN_SECTION.src_ip"
    uci -q delete "firewall.$LAN_SECTION.src_dip"
    uci -q delete "firewall.$LAN_SECTION.src_port"
    uci -q delete "firewall.$LAN_SECTION.dest"
    uci -q delete "firewall.$LAN_SECTION.dest_port_start"
    uci -q delete "firewall.$LAN_SECTION.dest_port_end"
}

ensure_redirect_local() {
    ensure_section "$LOCAL_SECTION" nat
    uci set "firewall.$LOCAL_SECTION.name=XRAY transparent proxy (local)"
    uci set "firewall.$LOCAL_SECTION.enabled=1"
    uci set "firewall.$LOCAL_SECTION.family=ipv4"
    uci set "firewall.$LOCAL_SECTION.table=nat"
    uci set "firewall.$LOCAL_SECTION.chain=output"
    uci set "firewall.$LOCAL_SECTION.proto=tcp"
    uci set "firewall.$LOCAL_SECTION.dest_ip=$SUBNET"
    uci set "firewall.$LOCAL_SECTION.target=redirect"
    uci set "firewall.$LOCAL_SECTION.dest_port=$PORT"
}

ensure_redirect_lan
ensure_redirect_local

uci commit firewall

if command -v fw4 >/dev/null 2>&1; then
    log "Reloading firewall via fw4"
    fw4 reload
else
    log "Reloading firewall service"
    /etc/init.d/firewall reload
fi

log "Firewall updated. TCP traffic for $SUBNET redirects to local port $PORT"
log "Zone handled: $ZONE"
