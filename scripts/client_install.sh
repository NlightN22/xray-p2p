#!/bin/sh
# Install XRAY client

log() {
    printf '%s\n' "$*" >&2
}

die() {
    printf 'Error: %s\n' "$*" >&2
    exit 1
}

usage() {
    cat <<'USAGE'
Usage: client_install.sh [TROJAN_URL]

Installs XRAY client configuration. The Trojan connection string can be
provided as the first argument, via one of the XRAY_TROJAN_URL,
XRAY_CLIENT_URL, XRAY_CONNECTION_URL or XRAY_CONNECTION_STRING environment
variables, or interactively when a terminal is available.
USAGE
}

CONNECTION_STRING=""
if [ "$#" -gt 0 ]; then
    case "$1" in
        -h|--help)
            usage
            exit 0
            ;;
        *)
            CONNECTION_STRING="$1"
            ;;
    esac
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
        log 'No connection string provided and no interactive terminal available; continuing without updating outbounds.json.'
    fi
}

if [ -z "$CONNECTION_STRING" ]; then
    prompt_connection_string
fi

if [ -z "$CONNECTION_STRING" ]; then
    log 'No connection string provided; default outbound configuration will remain in place.'
fi

INSTALL_SCRIPT_URL="https://gist.githubusercontent.com/NlightN22/d410a3f9dd674308999f13f3aeb558ff/raw/da2634081050deefd504504d5ecb86406381e366/install_xray_openwrt.sh"
TMP_INSTALL_SCRIPT=$(mktemp)
if ! curl -fsSL "$INSTALL_SCRIPT_URL" -o "$TMP_INSTALL_SCRIPT"; then
    rm -f "$TMP_INSTALL_SCRIPT"
    die "Failed to download XRAY installer script"
fi
if ! sh "$TMP_INSTALL_SCRIPT"; then
    rm -f "$TMP_INSTALL_SCRIPT"
    die "XRAY installer script execution failed"
fi
rm -f "$TMP_INSTALL_SCRIPT"

XRAY_CONFIG_DIR="/etc/xray"
if [ ! -d "$XRAY_CONFIG_DIR" ]; then
    log "Creating XRAY configuration directory at $XRAY_CONFIG_DIR"
    mkdir -p "$XRAY_CONFIG_DIR"
fi

CONFIG_BASE_URL="https://raw.githubusercontent.com/NlightN22/xray-p2p/main/config_templates/client"
CONFIG_FILES="inbounds.json logs.json outbounds.json"
for file in $CONFIG_FILES; do
    target="$XRAY_CONFIG_DIR/$file"
    url="$CONFIG_BASE_URL/$file"
    replace_file=1

    if [ -f "$target" ]; then
        case "${XRAY_FORCE_CONFIG:-}" in
            1)
                log "Replacing $target (forced by XRAY_FORCE_CONFIG=1)"
                ;;
            0)
                log "Keeping existing $target (XRAY_FORCE_CONFIG=0)"
                replace_file=0
                ;;
            *)
                while :; do
                    printf "File %s exists. Replace with repository version? [y/N]: " "$target" >&2
                    if [ -t 0 ]; then
                        IFS= read -r answer
                    elif [ -r /dev/tty ]; then
                        IFS= read -r answer </dev/tty
                    else
                        die "No interactive terminal available. Set XRAY_FORCE_CONFIG=1 to overwrite or 0 to keep existing files."
                    fi
                    case "$answer" in
                        [Yy]) replace_file=1; break ;;
                        [Nn]|"") replace_file=0; break ;;
                        *) log "Please answer y or n." ;;
                    esac
                done
                ;;
        esac
    fi

    if [ "$replace_file" -eq 0 ]; then
        log "Keeping existing $target"
        continue
    fi

    log "Downloading $file to $XRAY_CONFIG_DIR"
    if ! curl -fsSL "$url" -o "$target"; then
        die "Failed to download $file"
    fi
    chmod 644 "$target"
done

missing_deps=""
append_missing() {
    if [ -z "$missing_deps" ]; then
        missing_deps="$1"
    else
        missing_deps="$missing_deps\n$1"
    fi
}

if ! command -v uci >/dev/null 2>&1; then
    append_missing "- uci (required; ensure you are running this on OpenWrt)"
fi

if [ -n "$missing_deps" ]; then
    log "Missing required dependencies before continuing:"
    printf '%b\n' "$missing_deps" >&2
    die "Resolve missing dependencies and rerun the installer."
fi

XRAY_CONF_DIR_UCI="$(uci -q get xray.config.confdir 2>/dev/null)"
if [ -z "$XRAY_CONF_DIR_UCI" ]; then
    die "Unable to read xray.config.confdir via uci"
fi

if [ "$XRAY_CONF_DIR_UCI" != "$XRAY_CONFIG_DIR" ]; then
    log "UCI confdir ($XRAY_CONF_DIR_UCI) does not match expected path $XRAY_CONFIG_DIR"
    die "Update it with: uci set xray.config.confdir='$XRAY_CONFIG_DIR'; uci commit xray"
fi

uci_changes=0

if [ "$(uci -q get xray.enabled.enabled 2>/dev/null)" != "1" ]; then
    log "Enabling xray service to start on boot"
    uci set xray.enabled.enabled='1'
    uci_changes=1
fi

desired_conffiles="/etc/xray/inbounds.json /etc/xray/logs.json /etc/xray/outbounds.json"
existing_conffiles=$(uci -q show xray.config 2>/dev/null | awk -F= '/^xray.config.conffiles=/ {print $2}' | tr '\n' ' ' | sed 's/[[:space:]]*$//')

if [ "$existing_conffiles" != "$desired_conffiles" ]; then
    log "Aligning xray.config.conffiles with managed templates"
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
    die "Outbound configuration $OUTBOUND_FILE is missing"
fi

update_outbounds_from_connection() {
    url="$1"

    case "$url" in
        trojan://*) ;;
        *)
            die "Unsupported protocol in connection string. Expected trojan://"
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
        die "Connection string is missing password (expected password@host:port)"
    fi

    password_part="${base_part%%@*}"
    server_part="${base_part#*@}"

    if [ -z "$password_part" ]; then
        die "Password part of the connection string is empty"
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
                die "Connection string is missing port"
            fi
            port="${server_part##*:}"
            host="${server_part%:*}"
            ;;
    esac

    if [ -z "$host" ]; then
        die "Host portion of the connection string is empty"
    fi

    if [ -z "$port" ]; then
        die "Port portion of the connection string is empty"
    fi

    case "$port" in
        ''|*[!0-9]*)
            die "Port must be numeric"
            ;;
    esac

    port_num=$port
    if [ "$port_num" -le 0 ] || [ "$port_num" -gt 65535 ]; then
        die "Port must be between 1 and 65535"
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

    log "Updated $OUTBOUND_FILE with provided connection settings"
}

if [ -n "$CONNECTION_STRING" ]; then
    update_outbounds_from_connection "$CONNECTION_STRING"
fi

INBOUND_FILE="$XRAY_CONFIG_DIR/inbounds.json"
if [ ! -f "$INBOUND_FILE" ]; then
    die "Inbound configuration $INBOUND_FILE is missing"
fi

log "WARNING: dokodemo-door inbound will listen on all IPv4 addresses (0.0.0.0)"
log "WARNING: Restrict exposure with firewall rules if WAN access must be blocked"

SOCKS_PORT=$(awk 'match($0, /"port"[[:space:]]*:[[:space:]]*([0-9]+)/, m) {print m[1]; exit}' "$INBOUND_FILE")
if [ -z "$SOCKS_PORT" ]; then
    SOCKS_PORT=1080
fi

log "Restarting xray service"
if ! /etc/init.d/xray restart; then
    die "Failed to restart xray service"
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
    log "XRAY client is listening on local port $SOCKS_PORT"
elif [ "$port_check_status" -eq 2 ]; then
    log "XRAY service restarted. Skipping port verification because neither 'ss' nor 'netstat' is available."
    log "Install ip-full (ss) or net-tools-netstat to enable automatic checks."
else
    die "XRAY service does not appear to be listening on local port $SOCKS_PORT"
fi
