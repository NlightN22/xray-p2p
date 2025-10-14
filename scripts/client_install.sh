#!/bin/sh
# Install XRAY-P2P client (OpenWrt)

SCRIPT_NAME=${0##*/}

if [ -z "${XRAY_SELF_DIR:-}" ]; then
    case "$0" in
        */*)
            XRAY_SELF_DIR=$(CDPATH= cd -- "$(dirname "$0")" 2>/dev/null && pwd)
            export XRAY_SELF_DIR
            ;;
    esac
fi

# Ensure XRAY_SELF_DIR exists when invoked via stdin piping.
: "${XRAY_SELF_DIR:=}"

COMMON_LIB_REMOTE_PATH="scripts/lib/common.sh"

if ! command -v load_common_lib >/dev/null 2>&1; then
    for candidate in \
        "${XRAY_SELF_DIR%/}/scripts/lib/common_loader.sh" \
        "scripts/lib/common_loader.sh" \
        "lib/common_loader.sh"; do
        if [ -n "$candidate" ] && [ -r "$candidate" ]; then
            # shellcheck disable=SC1090
            . "$candidate"
            break
        fi
    done
fi

if ! command -v load_common_lib >/dev/null 2>&1; then
    base="${XRAY_REPO_BASE_URL:-https://raw.githubusercontent.com/NlightN22/xray-p2p/main}"
    loader_url="${base%/}/scripts/lib/common_loader.sh"
    tmp="$(mktemp 2>/dev/null)" || {
        printf 'Error: Unable to create temporary loader script.\n' >&2
        exit 1
    }
    if command -v curl >/dev/null 2>&1 && curl -fsSL "$loader_url" -o "$tmp"; then
        :
    elif command -v wget >/dev/null 2>&1 && wget -q -O "$tmp" "$loader_url"; then
        :
    else
        printf 'Error: Unable to download common loader from %s.\n' "$loader_url" >&2
        rm -f "$tmp"
        exit 1
    fi
    # shellcheck disable=SC1090
    . "$tmp"
    rm -f "$tmp"
fi

if ! command -v load_common_lib >/dev/null 2>&1; then
    printf 'Error: Unable to initialize XRAY common loader.\n' >&2
    exit 1
fi

if ! load_common_lib; then
    printf 'Error: Unable to load XRAY common library.\n' >&2
    exit 1
fi

umask 077

usage() {
    cat <<EOF
Usage: $SCRIPT_NAME [options] [TROJAN_URL]

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

REINSTALL=0
CONNECTION_STRING=""

while [ "$#" -gt 0 ]; do
    case "$1" in
        -h|--help)
            usage 0
            ;;
        -r|--reinstall)
            REINSTALL=1
            ;;
        --)
            shift
            break
            ;;
        -*)
            xray_log "Unknown option: $1"
            usage 1
            ;;
        *)
            if [ -n "$CONNECTION_STRING" ]; then
                xray_log "Unexpected argument: $1"
                usage 1
            fi
            CONNECTION_STRING="$1"
            shift
            continue
            ;;
    esac
    shift
done

if [ -z "$CONNECTION_STRING" ] && [ "$#" -gt 0 ]; then
    CONNECTION_STRING="$1"
    shift
fi


if [ "$#" -gt 0 ]; then
    xray_log "Unexpected argument: $1"
    usage 1
fi

if [ -z "$CONNECTION_STRING" ]; then
    for candidate_var in XRAY_TROJAN_URL XRAY_CLIENT_URL XRAY_CONNECTION_URL XRAY_CONNECTION_STRING; do
        eval "candidate_value=\${$candidate_var:-}"
        if [ -n "$candidate_value" ]; then
            CONNECTION_STRING="$candidate_value"
            break
        fi
    done
fi

prompt_connection_string() {
    if [ -t 0 ]; then
        printf 'Enter Trojan connection string (leave empty to skip): ' >&2
        IFS= read -r CONNECTION_STRING
    elif [ -r /dev/tty ]; then
        printf 'Enter Trojan connection string (leave empty to skip): ' >&2
        IFS= read -r CONNECTION_STRING </dev/tty
    else
        xray_log 'No connection string provided and no interactive terminal available; continuing without updating outbounds.json.'
    fi
}

if [ -z "$CONNECTION_STRING" ]; then
    prompt_connection_string
fi

if [ -z "$CONNECTION_STRING" ]; then
    xray_log 'No connection string provided; default outbound configuration will remain in place.'
fi

XRAY_ALREADY_INSTALLED=0
if command -v xray >/dev/null 2>&1; then
    XRAY_ALREADY_INSTALLED=1
fi

if [ "$REINSTALL" -eq 1 ] || [ "$XRAY_ALREADY_INSTALLED" -eq 0 ]; then
    INSTALL_SCRIPT_URL="https://gist.githubusercontent.com/NlightN22/d410a3f9dd674308999f13f3aeb558ff/raw/da2634081050deefd504504d5ecb86406381e366/install_xray_openwrt.sh"
    TMP_INSTALL_SCRIPT=$(mktemp 2>/dev/null) || xray_die "Unable to create temporary file for installer"
    if ! xray_download_file "$INSTALL_SCRIPT_URL" "$TMP_INSTALL_SCRIPT" "XRAY installer script"; then
        xray_die "Failed to download XRAY installer script"
    fi
    if ! sh "$TMP_INSTALL_SCRIPT"; then
        rm -f "$TMP_INSTALL_SCRIPT"
        xray_die "XRAY installer script execution failed"
    fi
    rm -f "$TMP_INSTALL_SCRIPT"
else
    xray_log "XRAY binary already detected; skipping installation (use --reinstall to force)."
fi

# Our dedicated config directory and service
XRAYP2P_CONFIG_DIR="/etc/xray-p2p"
XRAYP2P_DATA_DIR="/usr/share/xray-p2p"
XRAYP2P_SERVICE="/etc/init.d/xray-p2p"

CONFIG_FILES="inbounds.json logs.json outbounds.json"
XRAYP2P_NEEDS_SETUP=1
if [ -d "$XRAYP2P_CONFIG_DIR" ] && [ -f "$XRAYP2P_SERVICE" ] && [ -e "$XRAYP2P_DATA_DIR" ]; then
    XRAYP2P_NEEDS_SETUP=0
    for file in $CONFIG_FILES; do
        if [ ! -f "$XRAYP2P_CONFIG_DIR/$file" ]; then
            XRAYP2P_NEEDS_SETUP=1
            break
        fi
    done
fi
if [ "$REINSTALL" -eq 1 ]; then
    XRAYP2P_NEEDS_SETUP=1
fi

if [ "$XRAYP2P_NEEDS_SETUP" -eq 1 ]; then
    # Ensure config and data dirs
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

    # Seed our init script and UCI config
    xray_seed_file_from_template "$XRAYP2P_SERVICE" "config_templates/xray-p2p.init"
    chmod 0755 "$XRAYP2P_SERVICE" 2>/dev/null || true
    xray_seed_file_from_template "/etc/config/xray-p2p" "config_templates/xray-p2p.config"

    # Seed client JSONs into our directory
    for file in $CONFIG_FILES; do
        target="$XRAYP2P_CONFIG_DIR/$file"
        template_path="config_templates/client/$file"
        xray_seed_file_from_template "$target" "$template_path"
    done
else
    xray_log "xray-p2p service assets already detected; skipping template installation (use --reinstall to force)."
fi

xray_require_cmd jq

update_trojan_outbound() {
    file="$1"
    password="$2"
    address="$3"
    port="$4"
    server_name="$5"
    network="$6"
    security="$7"
    allow_insecure="$8"

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

OUTBOUND_FILE="$XRAYP2P_CONFIG_DIR/outbounds.json"
if [ ! -f "$OUTBOUND_FILE" ]; then
    xray_die "Outbound configuration $OUTBOUND_FILE is missing"
fi

update_outbounds_from_connection() {
    url="$1"

    case "$url" in
        trojan://*) ;;
        *)
            xray_die "Unsupported protocol in connection string. Expected trojan://"
            ;;
    esac

    without_proto="${url#trojan://}"

    main_part="$without_proto"
    case "$main_part" in
        *'#'*)
            main_part="${main_part%%#*}"
            ;;
    esac

    query=""
    base_part="$main_part"
    case "$main_part" in
        *'?'*)
            query="${main_part#*\?}"
            base_part="${main_part%%\?*}"
            ;;
    esac

    if [ "${base_part#*@}" = "$base_part" ]; then
        xray_die "Connection string is missing password (expected password@host:port)"
    fi

    password_part="${base_part%%@*}"
    server_part="${base_part#*@}"

    if [ -z "$password_part" ]; then
        xray_die "Password part of the connection string is empty"
    fi

    host=""
    port=""

    case "$server_part" in
        \[*\]*)
            host="${server_part%%]*}"
            host="${host#[}"
            remainder="${server_part#*]}"
            remainder="${remainder#*:}"
            port="$remainder"
            ;;
        *)
            if [ "${server_part##*:}" = "$server_part" ]; then
                xray_die "Connection string is missing port"
            fi
            port="${server_part##*:}"
            host="${server_part%:*}"
            ;;
    esac

    if [ -z "$host" ]; then
        xray_die "Host portion of the connection string is empty"
    fi

    if [ -z "$port" ]; then
        xray_die "Port portion of the connection string is empty"
    fi

    case "$port" in
        ''|*[!0-9]*)
            xray_die "Port must be numeric"
            ;;
    esac

    port_num=$port
    if [ "$port_num" -le 0 ] || [ "$port_num" -gt 65535 ]; then
        xray_die "Port must be between 1 and 65535"
    fi

    network_type="tcp"
    security_type="tls"
    allow_insecure_value="true"
    server_name="$host"

    remain="$query"
    while [ -n "$remain" ]; do
        case "$remain" in
            *'&'*)
                pair="${remain%%&*}"
                remain="${remain#*&}"
                ;;
            *)
                pair="$remain"
                remain=""
                ;;
        esac
        [ -z "$pair" ] && continue
        key="${pair%%=*}"
        value="${pair#*=}"
        if [ "$key" = "$pair" ]; then
            value=""
        fi
        case "$key" in
            type|network)
                [ -n "$value" ] && network_type="$value"
                ;;
            security)
                [ -n "$value" ] && security_type="$value"
                ;;
            allowInsecure)
                case "$value" in
                    1|true|TRUE|yes|on|enable|enabled)
                        allow_insecure_value="true"
                        ;;
                    0|false|FALSE|no|off|disable|disabled)
                        allow_insecure_value="false"
                        ;;
                esac
                ;;
            sni)
                if [ -n "$value" ]; then
                    server_name="$value"
                fi
                ;;
            peer)
                if [ -n "$value" ]; then
                    server_name="$value"
                fi
                ;;
        esac
    done

    if ! update_trojan_outbound "$OUTBOUND_FILE" "$password_part" "$host" "$port_num" "$server_name" "$network_type" "$security_type" "$allow_insecure_value"; then
        xray_die "Failed to update $OUTBOUND_FILE with provided connection settings"
    fi

    xray_log "Updated $OUTBOUND_FILE with provided connection settings"
}

if [ -n "$CONNECTION_STRING" ]; then
    update_outbounds_from_connection "$CONNECTION_STRING"
fi

INBOUND_FILE="$XRAYP2P_CONFIG_DIR/inbounds.json"
if [ ! -f "$INBOUND_FILE" ]; then
    xray_die "Inbound configuration $INBOUND_FILE is missing"
fi

xray_log "WARNING: dokodemo-door inbound will listen on all IPv4 addresses (0.0.0.0)"
xray_log "WARNING: Restrict exposure with firewall rules if WAN access must be blocked"

SOCKS_PORT=$(jq -r 'first(.inbounds[]? | select((.protocol // "") == "dokodemo-door") | .port) // empty' "$INBOUND_FILE" 2>/dev/null)
if [ -z "$SOCKS_PORT" ]; then
    SOCKS_PORT=1080
fi

"$XRAYP2P_SERVICE" enable >/dev/null 2>&1 || true
xray_restart_service "xray-p2p" "$XRAYP2P_SERVICE"

sleep 2

PORT_CHECK_CMD=""
if command -v ss >/dev/null 2>&1; then
    PORT_CHECK_CMD="ss"
elif command -v netstat >/dev/null 2>&1; then
    PORT_CHECK_CMD="netstat"
fi

check_port() {
    case "$PORT_CHECK_CMD" in
        ss)
            ss -ltn 2>/dev/null | grep -q ":$SOCKS_PORT "
            return $?
            ;;
        netstat)
            netstat -tln 2>/dev/null | grep -q ":$SOCKS_PORT "
            return $?
            ;;
        *)
            return 2
            ;;
    esac
}

check_port
port_check_status=$?
if [ "$port_check_status" -eq 0 ]; then
    xray_log "xray-p2p client is listening on local port $SOCKS_PORT"
elif [ "$port_check_status" -eq 2 ]; then
    xray_log "xray-p2p restarted. Skipping port verification because neither 'ss' nor 'netstat' is available."
    xray_log "Install ip-full (ss) or net-tools-netstat to enable automatic checks."
else
    xray_die "xray-p2p service does not appear to be listening on local port $SOCKS_PORT"
fi
