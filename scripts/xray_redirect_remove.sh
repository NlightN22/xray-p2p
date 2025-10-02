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
    if command -v nft >/dev/null 2>&1; then
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
    else
        log "nft binary not found; unable to remove runtime chains"
        if [ "$removed_snippet" -eq 0 ]; then
            die "No cleanup method available"
        fi
    fi
fi

log "Transparent redirect rules removed"
