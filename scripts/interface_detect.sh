#!/bin/sh

set -eu

log() {
    printf '%s\n' "$*"
}

die() {
    printf 'Error: %s\n' "$*" >&2
    exit 1
}

usage() {
    cat <<'USAGE'
Usage: interface_detect.sh TARGET

TARGET  IPv4 address or CIDR (e.g. 10.0.101.0/24) to test against.

Prints the local interface and source address the kernel would use
when reaching TARGET. CIDR inputs report the interface associated with
that prefix (via `ip route show match`).
USAGE
}

require_cmd() {
    cmd="$1"
    if ! command -v "$cmd" >/dev/null 2>&1; then
        die "Required command '$cmd' not found"
    fi
}

extract_field() {
    keyword="$1"
    awk -v key="$keyword" '{
        for (i = 1; i <= NF; i++) {
            if ($i == key && (i+1) <= NF) {
                print $(i+1)
                exit
            }
        }
    }'
}

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
    usage
    exit 0
fi

if [ "$#" -ne 1 ]; then
    usage
    exit 1
fi

require_cmd ip

TARGET="$1"
MODE="address"
case "$TARGET" in
    */*)
        MODE="cidr"
        ;;
esac

route_line=""
case "$MODE" in
    address)
        route_line=$(ip route get "$TARGET" 2>/dev/null | head -n1 || true)
        ;;
    cidr)
        route_line=$(ip route show match "$TARGET" 2>/dev/null | head -n1 || true)
        [ -z "$route_line" ] && route_line=$(ip route get "${TARGET%/*}" 2>/dev/null | head -n1 || true)
        ;;
esac

if [ -z "$route_line" ]; then
    die "Unable to determine route for $TARGET"
fi

iface=$(printf '%s\n' "$route_line" | extract_field dev || true)
if [ -z "$iface" ]; then
    die "Unable to extract interface from route information: $route_line"
fi

src_addr=$(printf '%s\n' "$route_line" | extract_field src || true)

log "Target: $TARGET"
log "Interface: $iface"
if [ -n "$src_addr" ]; then
    log "Source address: $src_addr"
fi
