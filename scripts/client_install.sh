#!/bin/sh
# Install XRAY client

SCRIPT_NAME=${0##*/}

if [ -z "${XRAY_SELF_DIR:-}" ]; then
    case "$0" in
        */*)
            XRAY_SELF_DIR=$(CDPATH= cd -- "$(dirname "$0")" 2>/dev/null && pwd)
            export XRAY_SELF_DIR
            ;;
    esac
fi

COMMON_LIB_REMOTE_PATH="scripts/lib/common.sh"

load_common_lib() {
    for candidate in \
        "${XRAY_SELF_DIR%/}/$COMMON_LIB_REMOTE_PATH" \
        "$COMMON_LIB_REMOTE_PATH" \
        "lib/common.sh"; do
        if [ -n "$candidate" ] && [ -r "$candidate" ]; then
            # shellcheck disable=SC1090
            . "$candidate"
            return 0
        fi
    done

    base="${XRAY_REPO_BASE_URL:-https://raw.githubusercontent.com/NlightN22/xray-p2p/main}"
    url="${base%/}/$COMMON_LIB_REMOTE_PATH"
    tmp="$(mktemp 2>/dev/null)" || {
        printf 'Error: Unable to create temporary file for common library.\n' >&2
        return 1
    }

    if command -v xray_download_file >/dev/null 2>&1; then
        if ! xray_download_file "$url" "$tmp" "common library"; then
            return 1
        fi
    else
        if command -v curl >/dev/null 2>&1 && curl -fsSL "$url" -o "$tmp"; then
            :
        elif command -v wget >/dev/null 2>&1 && wget -q -O "$tmp" "$url"; then
            :
        else
            printf 'Error: Unable to download common library from %s.\n' "$url" >&2
            rm -f "$tmp"
            return 1
        fi
    fi

    # shellcheck disable=SC1090
    . "$tmp"
    rm -f "$tmp"
    return 0
}

if ! load_common_lib; then
    printf 'Error: Unable to load XRAY common library.\n' >&2
    exit 1
fi

umask 077

usage() {
    cat <<EOF
Usage: $SCRIPT_NAME [options] [TROJAN_URL]

Install and configure the XRAY client. The optional TROJAN_URL argument
overrides environment variables and interactive input when provided.

Options:
  -h, --help        Show this help message and exit.

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

CONNECTION_STRING=""

while [ "$#" -gt 0 ]; do
    case "$1" in
        -h|--help)
            usage 0
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

XRAY_CONFIG_DIR="/etc/xray"
if [ ! -d "$XRAY_CONFIG_DIR" ]; then
    xray_log "Creating XRAY configuration directory at $XRAY_CONFIG_DIR"
    mkdir -p "$XRAY_CONFIG_DIR"
fi

CONFIG_FILES="inbounds.json logs.json outbounds.json"
for file in $CONFIG_FILES; do
    target="$XRAY_CONFIG_DIR/$file"
    template_path="config_templates/client/$file"
    xray_seed_file_from_template "$target" "$template_path"
done

xray_require_cmd uci

XRAY_CONF_DIR_UCI="$(uci -q get xray.config.confdir 2>/dev/null)"
if [ -z "$XRAY_CONF_DIR_UCI" ]; then
    xray_die "Unable to read xray.config.confdir via uci"
fi

if [ "$XRAY_CONF_DIR_UCI" != "$XRAY_CONFIG_DIR" ]; then
    xray_log "UCI confdir ($XRAY_CONF_DIR_UCI) does not match expected path $XRAY_CONFIG_DIR"
    xray_die "Update it with: uci set xray.config.confdir='$XRAY_CONFIG_DIR'; uci commit xray"
fi

uci_changes=0

if [ "$(uci -q get xray.enabled.enabled 2>/dev/null)" != "1" ]; then
    xray_log "Enabling xray service to start on boot"
    uci set xray.enabled.enabled='1'
    uci_changes=1
fi

desired_conffiles="/etc/xray/inbounds.json /etc/xray/logs.json /etc/xray/outbounds.json"
existing_conffiles=$(uci -q show xray.config 2>/dev/null | awk -F= '/^xray.config.conffiles=/ {print $2}' | tr '\n' ' ' | sed 's/[[:space:]]*$//')

if [ "$existing_conffiles" != "$desired_conffiles" ]; then
    xray_log "Aligning xray.config.conffiles with managed templates"
    uci -q delete xray.config.conffiles
    for file in $desired_conffiles; do
        uci add_list xray.config.conffiles="$file"
    done
    uci_changes=1
fi

if [ "$uci_changes" -eq 1 ]; then
    uci commit xray
fi

json_escape() {
    printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

replace_json_string() {
    file="$1"
    key="$2"
    value="$3"
    tmp_file=$(mktemp)
    awk -v key="$key" -v value="$value" '
        BEGIN {replaced=0}
        {
            if (!replaced && $0 ~ "\"" key "\"[[:space:]]*:") {
                sub("\"" key "\"[[:space:]]*:[[:space:]]*\"[^\"]*\"", "\"" key "\": \"" value "\"")
                replaced=1
            }
            print
        }
    ' "$file" > "$tmp_file" && mv "$tmp_file" "$file"
}

replace_json_number() {
    file="$1"
    key="$2"
    value="$3"
    tmp_file=$(mktemp)
    awk -v key="$key" -v value="$value" '
        BEGIN {replaced=0}
        {
            if (!replaced && $0 ~ "\"" key "\"[[:space:]]*:") {
                sub("\"" key "\"[[:space:]]*:[[:space:]]*[0-9]+", "\"" key "\": " value)
                replaced=1
            }
            print
        }
    ' "$file" > "$tmp_file" && mv "$tmp_file" "$file"
}

replace_json_bool() {
    file="$1"
    key="$2"
    value="$3"
    tmp_file=$(mktemp)
    awk -v key="$key" -v value="$value" '
        BEGIN {replaced=0}
        {
            if (!replaced && $0 ~ "\"" key "\"[[:space:]]*:") {
                sub("\"" key "\"[[:space:]]*:[[:space:]]*(true|false)", "\"" key "\": " value)
                replaced=1
            }
            print
        }
    ' "$file" > "$tmp_file" && mv "$tmp_file" "$file"
}

OUTBOUND_FILE="$XRAY_CONFIG_DIR/outbounds.json"
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

    escaped_password=$(json_escape "$password_part")
    escaped_host=$(json_escape "$host")
    escaped_server_name=$(json_escape "$server_name")
    escaped_network=$(json_escape "$network_type")
    escaped_security=$(json_escape "$security_type")

    replace_json_string "$OUTBOUND_FILE" "password" "$escaped_password"
    replace_json_string "$OUTBOUND_FILE" "address" "$escaped_host"
    replace_json_number "$OUTBOUND_FILE" "port" "$port_num"
    replace_json_string "$OUTBOUND_FILE" "serverName" "$escaped_server_name"
    replace_json_string "$OUTBOUND_FILE" "network" "$escaped_network"
    replace_json_string "$OUTBOUND_FILE" "security" "$escaped_security"
    replace_json_bool "$OUTBOUND_FILE" "allowInsecure" "$allow_insecure_value"

    xray_log "Updated $OUTBOUND_FILE with provided connection settings"
}

if [ -n "$CONNECTION_STRING" ]; then
    update_outbounds_from_connection "$CONNECTION_STRING"
fi

INBOUND_FILE="$XRAY_CONFIG_DIR/inbounds.json"
if [ ! -f "$INBOUND_FILE" ]; then
    xray_die "Inbound configuration $INBOUND_FILE is missing"
fi

xray_log "WARNING: dokodemo-door inbound will listen on all IPv4 addresses (0.0.0.0)"
xray_log "WARNING: Restrict exposure with firewall rules if WAN access must be blocked"

SOCKS_PORT=$(awk 'match($0, /"port"[[:space:]]*:[[:space:]]*([0-9]+)/, m) {print m[1]; exit}' "$INBOUND_FILE")
if [ -z "$SOCKS_PORT" ]; then
    SOCKS_PORT=1080
fi

xray_log "Restarting xray service"
if ! /etc/init.d/xray restart; then
    xray_die "Failed to restart xray service"
fi

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
    xray_log "XRAY client is listening on local port $SOCKS_PORT"
elif [ "$port_check_status" -eq 2 ]; then
    xray_log "XRAY service restarted. Skipping port verification because neither 'ss' nor 'netstat' is available."
    xray_log "Install ip-full (ss) or net-tools-netstat to enable automatic checks."
else
    xray_die "XRAY service does not appear to be listening on local port $SOCKS_PORT"
fi
