#!/bin/sh
# shellcheck shell=ash

[ "${SERVER_REMOVE_LIB_LOADED:-0}" = "1" ] && return 0
SERVER_REMOVE_LIB_LOADED=1

XRAYP2P_CONFIG_DIR="/etc/xray-p2p"
XRAYP2P_DATA_DIR="/usr/share/xray-p2p"
XRAYP2P_SERVICE="/etc/init.d/xray-p2p"
XRAYP2P_UCI_CONFIG="/etc/config/xray-p2p"

server_remove_run() {
    if [ "$#" -gt 0 ]; then
        xray_log "remove command does not accept additional arguments."
        exit 1
    fi

    if [ -x "$XRAYP2P_SERVICE" ]; then
        xray_log "Stopping xray-p2p service"
        "$XRAYP2P_SERVICE" stop >/dev/null 2>&1 || true
        "$XRAYP2P_SERVICE" disable >/dev/null 2>&1 || true
    fi

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

    if command -v opkg >/dev/null 2>&1; then
        if opkg list-installed xray-core 2>/dev/null | grep -q '^xray-core '; then
            xray_log "Removing xray-core package via opkg"
            if ! opkg remove xray-core >/dev/null 2>&1; then
                xray_warn "Failed to remove xray-core package; please remove it manually."
            fi
        fi
    fi

    xray_log "xray-p2p server installation removed."
}
