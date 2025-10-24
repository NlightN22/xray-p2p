#!/bin/sh
# shellcheck shell=ash

[ "${CLIENT_INSTALL_LIB_LOADED:-0}" = "1" ] && return 0
CLIENT_INSTALL_LIB_LOADED=1

XRAYP2P_CONFIG_DIR="/etc/xray-p2p"
XRAYP2P_DATA_DIR="/usr/share/xray-p2p"
XRAYP2P_SERVICE="/etc/init.d/xray-p2p"
XRAYP2P_UCI_CONFIG="/etc/config/xray-p2p"
CLIENT_INSTALLER_URL="https://gist.githubusercontent.com/NlightN22/d410a3f9dd674308999f13f3aeb558ff/raw/da2634081050deefd504504d5ecb86406381e366/install_xray_openwrt.sh"

if ! xray_common_try_source \
    "${XRAY_CLIENT_CONNECTION_LIB:-scripts/lib/client_connection.sh}" \
    "scripts/lib/client_connection.sh" \
    "lib/client_connection.sh"; then
    xray_die "Unable to load client connection library."
fi

client_install_usage() {
    cat <<EOF
Usage: ${SCRIPT_NAME:-client.sh}${CLIENT_INSTALL_USAGE_PREFIX:-} [options] [TROJAN_URL]

Install and configure XRAY binary and xray-p2p client config/service. The optional TROJAN_URL argument
overrides environment variables and interactive input when provided.

Options:
  -h, --help        Show this help message and exit.
  -r, --reinstall   Force reinstall of XRAY binary and xray-p2p assets.

Arguments:
  TROJAN_URL        Optional connection string; overrides env/prompt input.

Environment variables:
  XRAY_TROJAN_URL         Preferred Trojan/VLESS connection string.
  XRAY_CLIENT_URL         Alternative variable for compatibility.
  XRAY_CONNECTION_URL     Alternative variable for compatibility.
  XRAY_CONNECTION_STRING  Alternative variable for compatibility.
EOF
    exit "${1:-0}"
}

client_install_prompt_connection_string() {
    if [ -t 0 ]; then
        printf 'Enter Trojan connection string (leave empty to skip): ' >&2
        IFS= read -r CLIENT_INSTALL_CONNECTION_STRING
    elif [ -r /dev/tty ]; then
        printf 'Enter Trojan connection string (leave empty to skip): ' >&2
        IFS= read -r CLIENT_INSTALL_CONNECTION_STRING </dev/tty
    else
        xray_log 'No connection string provided and no interactive terminal available; continuing without updating outbounds.json.'
    fi
}

client_install_update_trojan_outbound() {
    local file="$1"
    local password="$2"
    local address="$3"
    local port="$4"
    local server_name="$5"
    local network="$6"
    local security="$7"
    local allow_insecure="$8"
    local tmp_file=""

    case "$allow_insecure" in
        true|false) : ;;
        *)
            allow_insecure="false"
            ;;
    esac

    tmp_file=$(mktemp) || return 1

    if ! jq \
        --arg password "$password" \
        --arg address "$address" \
        --arg serverName "$server_name" \
        --arg network "$network" \
        --arg security "$security" \
        --argjson port "$port" \
        --argjson allowInsecure "$allow_insecure" \
        '
        .outbounds |= (map(
            if (.protocol // "") == "trojan" then
                (.settings //= {})
                | (.settings.servers //= [{}])
                | (.settings.servers[0].address = $address)
                | (.settings.servers[0].port = $port)
                | (.settings.servers[0].password = $password)
                | (.streamSettings //= {})
                | (.streamSettings.network = $network)
                | (.streamSettings.security = $security)
                | (.streamSettings.tlsSettings //= {})
                | (.streamSettings.tlsSettings.serverName = $serverName)
                | (.streamSettings.tlsSettings.allowInsecure = $allowInsecure)
            else .
            end
        ))
        ' "$file" >"$tmp_file"; then
        rm -f "$tmp_file"
        return 1
    fi

    mv "$tmp_file" "$file"
}

client_install_update_outbounds_from_connection() {
    local file="$1"
    local url="$2"

    client_connection_parse "$url"

    if ! client_install_update_trojan_outbound \
        "$file" \
        "$CLIENT_CONNECTION_PASSWORD" \
        "$CLIENT_CONNECTION_HOST" \
        "$CLIENT_CONNECTION_PORT" \
        "$CLIENT_CONNECTION_SERVER_NAME" \
        "$CLIENT_CONNECTION_NETWORK" \
        "$CLIENT_CONNECTION_SECURITY" \
        "$CLIENT_CONNECTION_ALLOW_INSECURE"; then
        xray_die "Failed to update $file with provided connection settings"
    fi

    xray_log "Updated $file with provided connection settings"
}

client_install_check_port() {
    local port="$1"
    local checker="$2"

    case "$checker" in
        ss)
            ss -ltn 2>/dev/null | grep -q ":$port "
            return $?
            ;;
        netstat)
            netstat -tln 2>/dev/null | grep -q ":$port "
            return $?
            ;;
        *)
            return 2
            ;;
    esac
}

client_install_collect_ports() {
    local inbound_path="$1"
    jq -r '
        [.inbounds[]? | .port? | select((type == "number") or (type == "string" and test("^[0-9]+$"))) | tonumber]
        | unique
        | .[]
    ' "$inbound_path" 2>/dev/null
}

client_install_preflight_ports() {
    local inbound_path="$1"
    [ "${XRAY_SKIP_PORT_CHECK:-0}" = "1" ] && return 0

    local ports
    ports=$(client_install_collect_ports "$inbound_path")
    [ -n "$ports" ] || return 0

    local checker=""
    if command -v ss >/dev/null 2>&1; then
        checker="ss"
    elif command -v netstat >/dev/null 2>&1; then
        checker="netstat"
    else
        xray_log "Skipping preflight port check because neither 'ss' nor 'netstat' is available."
        return 0
    fi

    local collisions=""
    for port in $ports; do
        if client_install_check_port "$port" "$checker"; then
            collisions="${collisions:+$collisions }$port"
        fi
    done

    [ -z "$collisions" ] || xray_die "Required port(s) already in use: $collisions. Free these ports or set XRAY_SKIP_PORT_CHECK=1 to override."
}

client_install_run() {
    umask 077

    CLIENT_INSTALL_CONNECTION_STRING=""
    CLIENT_INSTALL_REINSTALL=0

    while [ "$#" -gt 0 ]; do
        case "$1" in
            -h|--help)
                client_install_usage 0
                ;;
            -r|--reinstall)
                CLIENT_INSTALL_REINSTALL=1
                ;;
            --)
                shift
                break
                ;;
            -*)
                xray_log "Unknown option: $1"
                client_install_usage 1
                ;;
            *)
                if [ -n "$CLIENT_INSTALL_CONNECTION_STRING" ]; then
                    xray_log "Unexpected argument: $1"
                    client_install_usage 1
                fi
                CLIENT_INSTALL_CONNECTION_STRING="$1"
                shift
                continue
                ;;
        esac
        shift
    done

    if [ -z "$CLIENT_INSTALL_CONNECTION_STRING" ] && [ "$#" -gt 0 ]; then
        CLIENT_INSTALL_CONNECTION_STRING="$1"
        shift
    fi

    if [ "$#" -gt 0 ]; then
        xray_log "Unexpected argument: $1"
        client_install_usage 1
    fi

    if [ -z "$CLIENT_INSTALL_CONNECTION_STRING" ]; then
        for candidate_var in XRAY_TROJAN_URL XRAY_CLIENT_URL XRAY_CONNECTION_URL XRAY_CONNECTION_STRING; do
            eval "client_install_candidate=\${$candidate_var:-}"
            if [ -n "$client_install_candidate" ]; then
                CLIENT_INSTALL_CONNECTION_STRING="$client_install_candidate"
                break
            fi
        done
        unset client_install_candidate
    fi

    if [ -z "$CLIENT_INSTALL_CONNECTION_STRING" ]; then
        client_install_prompt_connection_string
    fi

    if [ -z "$CLIENT_INSTALL_CONNECTION_STRING" ]; then
        xray_log 'No connection string provided; default outbound configuration will remain in place.'
    fi

    CLIENT_INSTALL_XRAY_ALREADY_INSTALLED=0
    if command -v xray >/dev/null 2>&1; then
        CLIENT_INSTALL_XRAY_ALREADY_INSTALLED=1
    fi

    if [ "$CLIENT_INSTALL_REINSTALL" -eq 1 ] || [ "$CLIENT_INSTALL_XRAY_ALREADY_INSTALLED" -eq 0 ]; then
        client_install_tmp_script=$(mktemp 2>/dev/null) || xray_die "Unable to create temporary file for installer"
        if ! xray_download_file "$CLIENT_INSTALLER_URL" "$client_install_tmp_script" "XRAY installer script"; then
            xray_die "Failed to download XRAY installer script"
        fi
        if ! sh "$client_install_tmp_script"; then
            rm -f "$client_install_tmp_script"
            xray_die "XRAY installer script execution failed"
        fi
        rm -f "$client_install_tmp_script"
    else
        xray_log "XRAY binary already detected; skipping installation (use --reinstall to force)."
    fi

    CLIENT_INSTALL_CONFIG_FILES="inbounds.json logs.json outbounds.json"
    CLIENT_INSTALL_NEEDS_SETUP=1
    if [ -d "$XRAYP2P_CONFIG_DIR" ] && [ -f "$XRAYP2P_SERVICE" ] && [ -e "$XRAYP2P_DATA_DIR" ]; then
        CLIENT_INSTALL_NEEDS_SETUP=0
        for file in $CLIENT_INSTALL_CONFIG_FILES; do
            if [ ! -f "$XRAYP2P_CONFIG_DIR/$file" ]; then
                CLIENT_INSTALL_NEEDS_SETUP=1
                break
            fi
        done
    fi
    if [ "$CLIENT_INSTALL_REINSTALL" -eq 1 ]; then
        CLIENT_INSTALL_NEEDS_SETUP=1
    fi

    if [ "$CLIENT_INSTALL_NEEDS_SETUP" -eq 1 ]; then
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

        xray_seed_file_from_template "$XRAYP2P_SERVICE" "config_templates/xray-p2p.init"
        chmod 0755 "$XRAYP2P_SERVICE" 2>/dev/null || true
        xray_seed_file_from_template "$XRAYP2P_UCI_CONFIG" "config_templates/xray-p2p.config"

        for file in $CLIENT_INSTALL_CONFIG_FILES; do
            target="$XRAYP2P_CONFIG_DIR/$file"
            template_path="config_templates/client/$file"
            xray_seed_file_from_template "$target" "$template_path"
        done
    else
        xray_log "xray-p2p service assets already detected; skipping template installation (use --reinstall to force)."
    fi

    xray_require_cmd jq

    CLIENT_INSTALL_OUTBOUND_FILE="$XRAYP2P_CONFIG_DIR/outbounds.json"
    if [ ! -f "$CLIENT_INSTALL_OUTBOUND_FILE" ]; then
        xray_die "Outbound configuration $CLIENT_INSTALL_OUTBOUND_FILE is missing"
    fi

    if [ -n "$CLIENT_INSTALL_CONNECTION_STRING" ]; then
        client_install_update_outbounds_from_connection "$CLIENT_INSTALL_OUTBOUND_FILE" "$CLIENT_INSTALL_CONNECTION_STRING"
    fi

    CLIENT_INSTALL_INBOUND_FILE="$XRAYP2P_CONFIG_DIR/inbounds.json"
    if [ ! -f "$CLIENT_INSTALL_INBOUND_FILE" ]; then
        xray_die "Inbound configuration $CLIENT_INSTALL_INBOUND_FILE is missing"
    fi

    xray_log "WARNING: dokodemo-door inbound will listen on all IPv4 addresses (0.0.0.0)"
    xray_log "WARNING: Restrict exposure with firewall rules if WAN access must be blocked"

    CLIENT_INSTALL_SOCKS_PORT=$(jq -r 'first(.inbounds[]? | select((.protocol // "") == "dokodemo-door") | .port) // empty' "$CLIENT_INSTALL_INBOUND_FILE" 2>/dev/null)
    if [ -z "$CLIENT_INSTALL_SOCKS_PORT" ]; then
        CLIENT_INSTALL_SOCKS_PORT=1080
    fi

    client_install_preflight_ports "$CLIENT_INSTALL_INBOUND_FILE"

    "$XRAYP2P_SERVICE" enable >/dev/null 2>&1 || true
    xray_restart_service "xray-p2p" "$XRAYP2P_SERVICE"

    sleep 2

    CLIENT_INSTALL_PORT_CHECKER=""
    if command -v ss >/dev/null 2>&1; then
        CLIENT_INSTALL_PORT_CHECKER="ss"
    elif command -v netstat >/dev/null 2>&1; then
        CLIENT_INSTALL_PORT_CHECKER="netstat"
    fi

    if [ "${XRAY_SKIP_PORT_CHECK:-0}" = "1" ]; then
        xray_log "Skipping client port verification because XRAY_SKIP_PORT_CHECK=1."
    else
        client_install_check_port "$CLIENT_INSTALL_SOCKS_PORT" "$CLIENT_INSTALL_PORT_CHECKER"
        client_install_port_status=$?
        if [ "$client_install_port_status" -eq 0 ]; then
            xray_log "xray-p2p client is listening on local port $CLIENT_INSTALL_SOCKS_PORT"
        elif [ "$client_install_port_status" -eq 2 ]; then
            xray_log "xray-p2p restarted. Skipping port verification because neither 'ss' nor 'netstat' is available."
            xray_log "Install ip-full (ss) or net-tools-netstat to enable automatic checks."
        else
            xray_die "xray-p2p service does not appear to be listening on local port $CLIENT_INSTALL_SOCKS_PORT"
        fi
    fi
}
