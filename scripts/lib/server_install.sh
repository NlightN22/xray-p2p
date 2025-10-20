#!/bin/sh
# shellcheck shell=ash

[ "${SERVER_INSTALL_LIB_LOADED:-0}" = "1" ] && return 0
SERVER_INSTALL_LIB_LOADED=1

XRAYP2P_CONFIG_DIR="/etc/xray-p2p"
XRAYP2P_DATA_DIR="/usr/share/xray-p2p"
XRAYP2P_SERVICE="/etc/init.d/xray-p2p"
XRAYP2P_UCI_CONFIG="/etc/config/xray-p2p"
SERVER_INSTALLER_URL="https://gist.githubusercontent.com/NlightN22/d410a3f9dd674308999f13f3aeb558ff/raw/da2634081050deefd504504d5ecb86406381e366/install_xray_openwrt.sh"
DEFAULT_XRAY_PORT=8443

server_install_tmp=""
server_install_inbound=""
server_install_port_arg=""
server_install_server_name_assigned=0

if ! xray_common_try_source \
    "${XRAY_SERVER_INSTALL_PORT_LIB:-scripts/lib/server_install_port.sh}" \
    "scripts/lib/server_install_port.sh" \
    "lib/server_install_port.sh"; then
    xray_die "Unable to load server install port helpers."
fi

if ! xray_common_try_source \
    "${XRAY_SERVER_INSTALL_SELF_CERT_LIB:-scripts/lib/server_install_cert_selfsigned.sh}" \
    "scripts/lib/server_install_cert_selfsigned.sh" \
    "lib/server_install_cert_selfsigned.sh"; then
    xray_die "Unable to load self-signed certificate helper."
fi

if ! xray_common_try_source \
    "${XRAY_SERVER_INSTALL_APPLY_CERT_LIB:-scripts/lib/server_install_cert_apply.sh}" \
    "scripts/lib/server_install_cert_apply.sh" \
    "lib/server_install_cert_apply.sh"; then
    xray_die "Unable to load certificate apply helper."
fi

server_install_cleanup() {
    [ -n "$server_install_tmp" ] && rm -f "$server_install_tmp"
    server_install_tmp=""
}

server_install_usage() {
    cat <<EOF
Usage: ${SCRIPT_NAME:-server.sh} install [options] [SERVER_NAME] [PORT]

Install and configure XRAY binary and xray-p2p service/config on OpenWrt.

Options:
  -h, --help        Show this help message and exit.

Arguments:
  SERVER_NAME      Optional TLS certificate Common Name; overrides env/prompt.
  PORT             Optional external port; overrides env/prompt (defaults 8443).

Environment variables:
  XRAY_FORCE_CONFIG     Set to 1 to overwrite config files, 0 to keep them.
  XRAY_PORT             Port to expose externally; prompts if unset (default 8443).
  XRAY_REISSUE_CERT     Set to 1 to regenerate TLS material, 0 to keep it.
  XRAY_SERVER_NAME      Common Name for generated TLS certificate.
  XRAY_SKIP_PORT_CHECK  Set to 1 to bypass preflight port availability validation.
EOF
    exit "${1:-0}"
}

server_install_parse_args() {
    while [ "$#" -gt 0 ]; do
        case "$1" in
            -h|--help)
                server_install_usage 0
                ;;
            --)
                shift
                break
                ;;
            -*)
                xray_log "Unknown option: $1"
                server_install_usage 1
                ;;
            *)
                if [ "$server_install_server_name_assigned" -eq 0 ]; then
                    XRAY_SERVER_NAME="$1"
                    server_install_server_name_assigned=1
                elif [ -z "$server_install_port_arg" ]; then
                    server_install_port_arg="$1"
                else
                    xray_log "Unexpected argument: $1"
                    server_install_usage 1
                fi
                ;;
        esac
        shift
    done

    if [ "$#" -gt 0 ]; then
        xray_log "Unexpected argument: $1"
        server_install_usage 1
    fi
}

server_install_fetch_installer() {
    server_install_tmp=$(mktemp 2>/dev/null) || xray_die "Unable to create temporary file for installer"
    trap server_install_cleanup EXIT INT TERM
    if ! xray_download_file "$SERVER_INSTALLER_URL" "$server_install_tmp" "XRAY installer script"; then
        xray_die "Failed to download XRAY installer script"
    fi
    if ! sh "$server_install_tmp"; then
        xray_die "XRAY installer script execution failed"
    fi
    server_install_cleanup
    trap - EXIT INT TERM
}

server_install_prepare_paths() {
    if [ ! -d "$XRAYP2P_CONFIG_DIR" ]; then
        xray_log "Creating xray-p2p configuration directory at $XRAYP2P_CONFIG_DIR"
        mkdir -p "$XRAYP2P_CONFIG_DIR"
    fi
    if [ ! -e "$XRAYP2P_DATA_DIR" ]; then
        if [ -d "/usr/share/xray" ]; then
            ln -s "/usr/share/xray" "$XRAYP2P_DATA_DIR" 2>/dev/null || mkdir -p "$XRAYP2P_DATA_DIR"
        else
            mkdir -p "$XRAYP2P_DATA_DIR"
        fi
    fi
}

server_install_seed_templates() {
    xray_seed_file_from_template "$XRAYP2P_SERVICE" "config_templates/xray-p2p.init"
    chmod 0755 "$XRAYP2P_SERVICE" 2>/dev/null || true
    xray_seed_file_from_template "$XRAYP2P_UCI_CONFIG" "config_templates/xray-p2p.config"

    for file in inbounds.json logs.json outbounds.json; do
        xray_seed_file_from_template "$XRAYP2P_CONFIG_DIR/$file" "config_templates/server/$file"
    done

    server_install_inbound="$XRAYP2P_CONFIG_DIR/inbounds.json"
    [ -f "$server_install_inbound" ] || xray_die "Inbound configuration $server_install_inbound is missing"
}

server_install_require_tools() {
    xray_require_cmd jq
}

server_install_enable_service() {
    "$XRAYP2P_SERVICE" enable >/dev/null 2>&1 || true
    xray_restart_service "xray-p2p" "$XRAYP2P_SERVICE"
    sleep 2
}

server_install_verify_service() {
    check_cmd=""
    if command -v ss >/dev/null 2>&1; then
        check_cmd="ss"
    elif command -v netstat >/dev/null 2>&1; then
        check_cmd="netstat"
    fi

    server_install_check_listen() {
        case "$check_cmd" in
            ss)
                ss -ltn 2>/dev/null | grep -q ":$XRAY_PORT "
                return $?
                ;;
            netstat)
                netstat -tln 2>/dev/null | grep -q ":$XRAY_PORT "
                return $?
                ;;
            *)
                return 2
                ;;
        esac
    }

    server_install_check_listen
    status=$?
    if [ "$status" -eq 0 ]; then
        xray_log "xray-p2p service is listening on port $XRAY_PORT"
    elif [ "$status" -eq 2 ]; then
        xray_log "xray-p2p restarted. Skipping port verification because neither 'ss' nor 'netstat' is available."
        xray_log "Install ip-full (ss) or net-tools-netstat to enable automatic checks."
    else
        xray_die "xray-p2p service does not appear to be listening on port $XRAY_PORT"
    fi
}

server_install_run() {
    umask 077
    server_install_parse_args "$@"
    server_install_fetch_installer
    server_install_prepare_paths
    server_install_seed_templates
    server_install_require_tools
    server_install_determine_port "$server_install_port_arg"
    server_install_update_inbound "$server_install_inbound" "$XRAY_PORT"
    server_install_preflight_ports "$server_install_inbound"
    cert_applied=0
    if [ -n "${XRAY_CERT_FILE:-}" ] || [ -n "${XRAY_KEY_FILE:-}" ]; then
        if [ -z "${XRAY_CERT_FILE:-}" ] || [ -z "${XRAY_KEY_FILE:-}" ]; then
            xray_warn "Both --cert and --key must be provided; falling back to self-signed certificate."
        elif server_install_cert_apply_paths "$server_install_inbound" "$XRAY_CERT_FILE" "$XRAY_KEY_FILE"; then
            cert_applied=1
        else
            xray_warn "Failed to apply provided certificate/key; generating self-signed certificate instead."
        fi
    fi
    if [ "$cert_applied" -ne 1 ]; then
        server_install_selfsigned_handle "$server_install_inbound"
    fi
    server_install_enable_service
    server_install_verify_service
}
