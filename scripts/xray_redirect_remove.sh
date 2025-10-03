#!/bin/sh

set -eu

NFT_SNIPPET="/etc/nftables.d/xray-transparent.nft"

log() {
    printf '%s\n' "$*"
}

die() {
    printf 'Error: %s\n' "$*" >&2
    exit 1
}

require_cmd() {
    cmd="$1"
    if command -v "$cmd" >/dev/null 2>&1; then
        return
    fi

    case "$cmd" in
        nft)
            die "Required command 'nft' not found. Install nftables (e.g. opkg update && opkg install nftables)."
            ;;
        *)
            die "Required command '$cmd' not found. Install it before running this script."
            ;;
    esac
}

usage() {
    cat <<'USAGE'
Usage: xray_redirect_remove.sh

Removes the nftables snippet installed by xray_redirect.sh and flushes the
runtime chains that implement the transparent redirect.
USAGE
}

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
    usage
    exit 0
fi

if [ "$#" -gt 0 ]; then
    usage
    exit 1
fi

removed_snippet=0
if [ -f "$NFT_SNIPPET" ]; then
    rm -f "$NFT_SNIPPET"
    removed_snippet=1
    log "Removed nftables snippet $NFT_SNIPPET"
else
    log "Snippet $NFT_SNIPPET not present"
fi

fw4_ok=0
if command -v fw4 >/dev/null 2>&1; then
    if fw4 reload >/dev/null 2>&1; then
        fw4_ok=1
        log "fw4 reload ok"
    else
        log "fw4 reload failed; attempting direct nft cleanup"
    fi
else
    log "fw4 binary not found; attempting direct nft cleanup"
fi

if [ "$fw4_ok" -eq 0 ]; then
    require_cmd nft

    delete_chain() {
        chain_name="$1"
        if nft list chain inet fw4 "$chain_name" >/dev/null 2>&1; then
            nft flush chain inet fw4 "$chain_name" >/dev/null 2>&1 || true
            if nft delete chain inet fw4 "$chain_name" >/dev/null 2>&1; then
                log "Removed chain inet fw4 $chain_name"
            else
                log "Failed to delete chain inet fw4 $chain_name"
            fi
        fi
    }

    delete_chain xray_transparent_prerouting
    delete_chain xray_transparent_output
fi

log "Transparent redirect rules removed"
