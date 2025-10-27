#!/bin/sh
# shellcheck shell=ash

[ "${CLIENT_REMOVE_LIB_LOADED:-0}" = "1" ] && return 0
CLIENT_REMOVE_LIB_LOADED=1

XRAYP2P_CONFIG_DIR="/etc/xray-p2p"
XRAYP2P_DATA_DIR="/usr/share/xray-p2p"
XRAYP2P_SERVICE="/etc/init.d/xray-p2p"
XRAYP2P_UCI_CONFIG="/etc/config/xray-p2p"

client_remove_clear_redirects() {
    # Wipe nftables redirect entries so subsequent installs start clean.
    if ! command -v xray_run_repo_script >/dev/null 2>&1; then
        return
    fi

    if (XRAY_SKIP_REPO_CHECK=1 XRAY_FORCE_CONFIG=1 \
        xray_run_repo_script optional \
            "scripts/redirect.sh" "scripts/redirect.sh" remove --all >/dev/null 2>&1); then
        xray_log "Cleared transparent redirect entries."
    fi
}

client_remove_usage() {
    cat <<EOF
Usage: ${SCRIPT_NAME:-client.sh} remove [--purge-core]

Remove xray-p2p client service, configuration, and data. Optional --purge-core removes the xray-core package.
EOF
    exit "${1:-0}"
}

client_remove_run() {
    local purge_core=0

    while [ "$#" -gt 0 ]; do
        case "$1" in
            --purge-core)
                purge_core=1
                ;;
            -h|--help)
                client_remove_usage 0
                ;;
            -*)
                xray_log "Unknown option: $1"
                client_remove_usage 1
                ;;
            *)
                xray_log "Unexpected argument: $1"
                client_remove_usage 1
                ;;
        esac
        shift
    done

    if [ -x "$XRAYP2P_SERVICE" ]; then
        xray_log "Stopping xray-p2p service"
        "$XRAYP2P_SERVICE" stop >/dev/null 2>&1 || true
        "$XRAYP2P_SERVICE" disable >/dev/null 2>&1 || true
    fi

    client_remove_clear_redirects

    if [ -e "$XRAYP2P_SERVICE" ]; then
        xray_log "Removing service script $XRAYP2P_SERVICE"
        rm -f "$XRAYP2P_SERVICE" || xray_warn "Unable to remove $XRAYP2P_SERVICE"
    fi

    if [ -e "$XRAYP2P_UCI_CONFIG" ]; then
        xray_log "Removing UCI configuration $XRAYP2P_UCI_CONFIG"
        rm -f "$XRAYP2P_UCI_CONFIG" || xray_warn "Unable to remove $XRAYP2P_UCI_CONFIG"
    fi

    if [ -d "$XRAYP2P_CONFIG_DIR" ] || [ -e "$XRAYP2P_CONFIG_DIR" ]; then
        xray_log "Removing configuration directory $XRAYP2P_CONFIG_DIR"
        rm -rf "$XRAYP2P_CONFIG_DIR" || xray_warn "Unable to remove $XRAYP2P_CONFIG_DIR"
    fi

    if [ -d "$XRAYP2P_DATA_DIR" ] || [ -L "$XRAYP2P_DATA_DIR" ]; then
        xray_log "Removing data directory $XRAYP2P_DATA_DIR"
        rm -rf "$XRAYP2P_DATA_DIR" || xray_warn "Unable to remove $XRAYP2P_DATA_DIR"
    fi

    if [ "$purge_core" -eq 1 ] && command -v opkg >/dev/null 2>&1; then
        if opkg list-installed xray-core 2>/dev/null | grep -q '^xray-core '; then
            xray_log "Removing xray-core package via opkg"
            if ! opkg remove xray-core >/dev/null 2>&1; then
                xray_warn "Failed to remove xray-core package; please remove it manually."
            fi
        fi
    else
        xray_log "Keeping xray-core package; pass --purge-core to remove it."
    fi

    xray_log "xray-p2p client installation removed."

    client_remove_cleanup_cache
}

client_remove_cleanup_cache() {
    seen=""
    for var in XRAY_LIB_CACHE_DIR XRAY_SCRIPT_ROOT XRAY_SELF_DIR; do
        eval "candidate=\${$var:-}"
        [ -n "$candidate" ] || continue
        case " $seen " in
            *" $candidate "*)
                continue
                ;;
        esac
        case "$candidate" in
            /tmp/*|/var/tmp/*)
                if [ -e "$candidate" ]; then
                    xray_log "Removing loader cache directory $candidate"
                    rm -rf "$candidate" || xray_warn "Unable to remove $candidate"
                fi
                ;;
        esac
        seen="$seen $candidate"
    done
    unset XRAY_LIB_CACHE_DIR XRAY_SCRIPT_ROOT XRAY_SELF_DIR
}
