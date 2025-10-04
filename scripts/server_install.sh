#!/bin/sh
# Install XRAY server

SCRIPT_NAME=${0##*/}

log() {
    printf '%s\n' "$*" >&2
}

die() {
    printf 'Error: %s\n' "$*" >&2
    exit 1
}

usage() {
    cat <<EOF
Usage: $SCRIPT_NAME [options] [SERVER_NAME]

Install and configure the XRAY server on OpenWrt.

Options:
  -h, --help        Show this help message and exit.

Arguments:
  SERVER_NAME      Optional TLS certificate Common Name; overrides env/prompt.

Environment variables:
  XRAY_FORCE_CONFIG     Set to 1 to overwrite config files, 0 to keep them.
  XRAY_PORT             Port to expose externally; prompts if unset.
  XRAY_REISSUE_CERT     Set to 1 to regenerate TLS material, 0 to keep it.
  XRAY_SERVER_NAME      Common Name for generated TLS certificate.
EOF
    exit "${1:-0}"
}

server_name_assigned=0

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
            log "Unknown option: $1"
            usage 1
            ;;
        *)
            if [ "${server_name_assigned:-0}" -eq 1 ]; then
                log "Unexpected argument: $1"
                usage 1
            fi
            XRAY_SERVER_NAME="$1"
            server_name_assigned=1
            shift
            continue
            ;;
    esac
    shift
done

if [ "$#" -gt 0 ]; then
    log "Unexpected argument: $1"
    usage 1
fi

curl -s https://gist.githubusercontent.com/NlightN22/d410a3f9dd674308999f13f3aeb558ff/raw/da2634081050deefd504504d5ecb86406381e366/install_xray_openwrt.sh | sh

XRAY_CONFIG_DIR="/etc/xray"
if [ ! -d "$XRAY_CONFIG_DIR" ]; then
    log "Creating XRAY configuration directory at $XRAY_CONFIG_DIR"
    mkdir -p "$XRAY_CONFIG_DIR"
fi

CONFIG_BASE_URL="https://raw.githubusercontent.com/NlightN22/xray-p2p/main/config_templates/server"
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

INBOUND_FILE="$XRAY_CONFIG_DIR/inbounds.json"
if [ ! -f "$INBOUND_FILE" ]; then
    die "Inbound configuration $INBOUND_FILE is missing"
fi

CERT_FILE=$(awk -F'"' '/"certificateFile"/ {print $4; exit}' "$INBOUND_FILE")
KEY_FILE=$(awk -F'"' '/"keyFile"/ {print $4; exit}' "$INBOUND_FILE")

[ -z "$CERT_FILE" ] && CERT_FILE="$XRAY_CONFIG_DIR/cert.pem"
[ -z "$KEY_FILE" ] && KEY_FILE="$XRAY_CONFIG_DIR/key.pem"

CERT_DIR=$(dirname "$CERT_FILE")
KEY_DIR=$(dirname "$KEY_FILE")

CERT_EXISTS=0
[ -f "$CERT_FILE" ] && CERT_EXISTS=1

KEY_EXISTS=0
[ -f "$KEY_FILE" ] && KEY_EXISTS=1

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

require_openssl=0
case "${XRAY_REISSUE_CERT:-}" in
    1)
        require_openssl=1
        ;;
    0)
        if [ "$CERT_EXISTS" -eq 0 ] || [ "$KEY_EXISTS" -eq 0 ]; then
            require_openssl=1
        fi
        ;;
    *)
        if [ "$CERT_EXISTS" -eq 0 ] || [ "$KEY_EXISTS" -eq 0 ]; then
            require_openssl=1
        fi
        ;;
esac

if [ "$require_openssl" -eq 1 ] && ! command -v openssl >/dev/null 2>&1; then
    append_missing "- openssl (install with: opkg update && opkg install openssl-util)"
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

DEFAULT_PORT=8443

if [ -n "$XRAY_PORT" ]; then
    log "Using XRAY_PORT=$XRAY_PORT from environment"
else
    printf "Enter external port for XRAY [%s]: " "$DEFAULT_PORT" >&2
    if [ -t 0 ]; then
        IFS= read -r XRAY_PORT
    elif [ -r /dev/tty ]; then
        IFS= read -r XRAY_PORT </dev/tty
    else
        die "No interactive terminal available. Set XRAY_PORT environment variable."
    fi
fi

if [ -z "$XRAY_PORT" ]; then
    XRAY_PORT="$DEFAULT_PORT"
fi

if ! echo "$XRAY_PORT" | grep -Eq "^[0-9]+$"; then
    die "Port must be numeric"
fi

if [ "$XRAY_PORT" -le 0 ] || [ "$XRAY_PORT" -gt 65535 ]; then
    die "Port must be between 1 and 65535"
fi

tmp_inbound="${INBOUND_FILE}.tmp"
awk -v port="$XRAY_PORT" '
    BEGIN {replaced=0}
    /"port"[[:space:]]*:/ && !replaced {
        sub(/"port"[[:space:]]*:[[:space:]]*[0-9]+/, "\"port\": " port)
        replaced=1
    }
    {print}
' "$INBOUND_FILE" > "$tmp_inbound" && mv "$tmp_inbound" "$INBOUND_FILE"

if ! grep -q "\"port\": $XRAY_PORT" "$INBOUND_FILE"; then
    die "Failed to update port in $INBOUND_FILE"
fi

reissue_cert=1
if [ -f "$CERT_FILE" ] || [ -f "$KEY_FILE" ]; then
    case "${XRAY_REISSUE_CERT:-}" in
        1)
            log "Regenerating certificate and key (forced by XRAY_REISSUE_CERT=1)"
            ;;
        0)
            log "Keeping existing certificate and key (XRAY_REISSUE_CERT=0)"
            reissue_cert=0
            ;;
        *)
            while :; do
                printf "Certificate or key already exists. Regenerate them now? [y/N]: " >&2
                if [ -t 0 ]; then
                    IFS= read -r cert_answer
                elif [ -r /dev/tty ]; then
                    IFS= read -r cert_answer </dev/tty
                else
                    die "No interactive terminal available. Set XRAY_REISSUE_CERT=1 to regenerate or 0 to keep existing material."
                fi
                case "$cert_answer" in
                    [Yy]) reissue_cert=1; break ;;
                    [Nn]|"") reissue_cert=0; break ;;
                    *) log "Please answer y or n." ;;
                esac
            done
            ;;
    esac
elif [ "${XRAY_REISSUE_CERT:-}" = "0" ]; then
    log "Certificate files are missing; generating new ones despite XRAY_REISSUE_CERT=0."
fi

if [ "$reissue_cert" -eq 1 ]; then
    if ! command -v openssl >/dev/null 2>&1; then
        die "openssl binary is required to generate certificates (install package openssl-util)."
    fi

    if [ -n "$XRAY_SERVER_NAME" ]; then
        XRAY_CERT_NAME="$XRAY_SERVER_NAME"
        log "Using XRAY_SERVER_NAME=$XRAY_CERT_NAME from environment"
    else
        EXISTING_CERT_CN=""
        if [ "$CERT_EXISTS" -eq 1 ] && command -v openssl >/dev/null 2>&1; then
            EXISTING_CERT_CN=$(openssl x509 -noout -subject -nameopt RFC2253 -in "$CERT_FILE" 2>/dev/null | awk -F'CN=' 'NF>1 {print $2}' | cut -d',' -f1 | sed 's/^ *//;s/ *$//')
        fi

        XRAY_CERT_NAME=""
        while [ -z "$XRAY_CERT_NAME" ]; do
            if [ -n "$EXISTING_CERT_CN" ]; then
                printf "Enter server name for TLS certificate [%s]: " "$EXISTING_CERT_CN" >&2
            else
                printf "Enter server name for TLS certificate (e.g. vpn.example.com): " >&2
            fi

            if [ -t 0 ]; then
                IFS= read -r XRAY_CERT_NAME
            elif [ -r /dev/tty ]; then
                IFS= read -r XRAY_CERT_NAME </dev/tty
            else
                die "No interactive terminal available. Set XRAY_SERVER_NAME environment variable."
            fi

            if [ -z "$XRAY_CERT_NAME" ] && [ -n "$EXISTING_CERT_CN" ]; then
                XRAY_CERT_NAME="$EXISTING_CERT_CN"
            fi

            if [ -z "$XRAY_CERT_NAME" ]; then
                log "Server name cannot be empty."
            elif ! echo "$XRAY_CERT_NAME" | grep -Eq "^[A-Za-z0-9.-]+$"; then
                log "Server name must contain only letters, digits, dots or hyphens."
                XRAY_CERT_NAME=""
            fi
        done
    fi

    if ! echo "$XRAY_CERT_NAME" | grep -Eq "^[A-Za-z0-9.-]+$"; then
        die "Server name must contain only letters, digits, dots or hyphens."
    fi

    mkdir -p "$CERT_DIR" "$KEY_DIR"

    BACKUP_SUFFIX=$(date +%Y%m%d%H%M%S)
    if [ -f "$CERT_FILE" ]; then
        log "Backing up existing certificate to ${CERT_FILE}.${BACKUP_SUFFIX}.bak"
        mv "$CERT_FILE" "${CERT_FILE}.${BACKUP_SUFFIX}.bak"
    fi
    if [ -f "$KEY_FILE" ]; then
        log "Backing up existing key to ${KEY_FILE}.${BACKUP_SUFFIX}.bak"
        mv "$KEY_FILE" "${KEY_FILE}.${BACKUP_SUFFIX}.bak"
    fi

    OPENSSL_CNF=$(mktemp)
    cat > "$OPENSSL_CNF" <<EOF
[req]
prompt = no
default_bits = 2048
default_md = sha256
req_extensions = req_ext
distinguished_name = dn

[dn]
CN = $XRAY_CERT_NAME

[req_ext]
subjectAltName = @alt_names

[alt_names]
DNS.1 = $XRAY_CERT_NAME
EOF

    if ! openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout "$KEY_FILE" -out "$CERT_FILE" -config "$OPENSSL_CNF" >/dev/null 2>&1; then
        rm -f "$OPENSSL_CNF"
        die "Failed to generate certificate for $XRAY_CERT_NAME"
    fi
    rm -f "$OPENSSL_CNF"

    chmod 600 "$KEY_FILE"
    chmod 644 "$CERT_FILE"
else
    log "Skipping certificate regeneration; keeping existing files in place."
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

check_port
port_check_status=$?
if [ "$port_check_status" -eq 0 ]; then
    log "XRAY service is listening on port $XRAY_PORT"
elif [ "$port_check_status" -eq 2 ]; then
    log "XRAY service restarted. Skipping port verification because neither 'ss' nor 'netstat' is available."
    log "Install ip-full (ss) or net-tools-netstat to enable automatic checks."
else
    die "XRAY service does not appear to be listening on port $XRAY_PORT"
fi
