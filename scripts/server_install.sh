#!/bin/sh
# Install XRAY-P2P server (OpenWrt)

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
Usage: $SCRIPT_NAME [options] [SERVER_NAME] [PORT]

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
EOF
    exit "${1:-0}"
}

server_name_assigned=0
port_arg=""

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
            if [ "${server_name_assigned:-0}" -eq 0 ]; then
                XRAY_SERVER_NAME="$1"
                server_name_assigned=1
            elif [ -z "$port_arg" ]; then
                port_arg="$1"
            else
                xray_log "Unexpected argument: $1"
                usage 1
            fi
            ;;
    esac
    shift
done

if [ "$#" -gt 0 ]; then
    xray_log "Unexpected argument: $1"
    usage 1
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

# Our dedicated config directory and service
XRAYP2P_CONFIG_DIR="/etc/xray-p2p"
XRAYP2P_DATA_DIR="/usr/share/xray-p2p"
XRAYP2P_SERVICE="/etc/init.d/xray-p2p"

# Ensure config and data dirs
if [ ! -d "$XRAYP2P_CONFIG_DIR" ]; then
    xray_log "Creating xray-p2p configuration directory at $XRAYP2P_CONFIG_DIR"
    mkdir -p "$XRAYP2P_CONFIG_DIR"
fi
if [ ! -e "$XRAYP2P_DATA_DIR" ]; then
    # If the standard asset dir exists, point ours to it via symlink to avoid duplication
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

# Seed server JSONs into our directory
CONFIG_FILES="inbounds.json logs.json outbounds.json"
for file in $CONFIG_FILES; do
    target="$XRAYP2P_CONFIG_DIR/$file"
    template_path="config_templates/server/$file"
    xray_seed_file_from_template "$target" "$template_path"
done

INBOUND_FILE="$XRAYP2P_CONFIG_DIR/inbounds.json"
if [ ! -f "$INBOUND_FILE" ]; then
    xray_die "Inbound configuration $INBOUND_FILE is missing"
fi

xray_require_cmd jq

CERT_FILE=$(jq -r 'first(.inbounds[]? | .streamSettings? | .tlsSettings? | .certificates[]? | .certificateFile) // empty' "$INBOUND_FILE" 2>/dev/null)
KEY_FILE=$(jq -r 'first(.inbounds[]? | .streamSettings? | .tlsSettings? | .certificates[]? | .keyFile) // empty' "$INBOUND_FILE" 2>/dev/null)

[ -z "$CERT_FILE" ] && CERT_FILE="$XRAYP2P_CONFIG_DIR/cert.pem"
[ -z "$KEY_FILE" ] && KEY_FILE="$XRAYP2P_CONFIG_DIR/key.pem"

CERT_DIR=$(dirname "$CERT_FILE")
KEY_DIR=$(dirname "$KEY_FILE")

CERT_EXISTS=0
[ -f "$CERT_FILE" ] && CERT_EXISTS=1

KEY_EXISTS=0
[ -f "$KEY_FILE" ] && KEY_EXISTS=1

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

if [ "$require_openssl" -eq 1 ]; then
    xray_require_cmd openssl
fi

:

DEFAULT_PORT=8443

if [ -n "$port_arg" ]; then
    XRAY_PORT="$port_arg"
elif [ -n "$XRAY_PORT" ]; then
    xray_log "Using XRAY_PORT=$XRAY_PORT from environment"
else
    printf "Enter external port for XRAY [%s]: " "$DEFAULT_PORT" >&2
    if [ -t 0 ]; then
        IFS= read -r XRAY_PORT
    elif [ -r /dev/tty ]; then
        IFS= read -r XRAY_PORT </dev/tty
    else
        xray_die "No interactive terminal available. Provide port as argument or set XRAY_PORT."
    fi
fi

if [ -z "$XRAY_PORT" ]; then
    XRAY_PORT="$DEFAULT_PORT"
fi

if ! echo "$XRAY_PORT" | grep -Eq "^[0-9]+$"; then
    xray_die "Port must be numeric"
fi

if [ "$XRAY_PORT" -le 0 ] || [ "$XRAY_PORT" -gt 65535 ]; then
    xray_die "Port must be between 1 and 65535"
fi

tmp_inbound=$(mktemp) || xray_die "Unable to create temporary file for inbound update"
if ! jq --argjson port "$XRAY_PORT" --arg cert "$CERT_FILE" --arg key "$KEY_FILE" '
    .inbounds |= (map(
        if (.protocol // "") == "trojan" then
            .port = $port
            | .streamSettings.tlsSettings.certificates |= (map(
                .certificateFile = $cert | .keyFile = $key
            ))
        else .
        end
    ))
' "$INBOUND_FILE" >"$tmp_inbound"; then
    rm -f "$tmp_inbound"
    xray_die "Failed to update inbound settings"
fi
mv "$tmp_inbound" "$INBOUND_FILE"
if ! jq -e --argjson port "$XRAY_PORT" --arg cert "$CERT_FILE" --arg key "$KEY_FILE" 'any(.inbounds[]?; (.protocol // "") == "trojan" and (.port // 0) == $port) and any(.inbounds[]?; (.streamSettings? // {} | .tlsSettings? // {} | .certificates[]? // {} | (.certificateFile? == $cert and .keyFile? == $key)))' "$INBOUND_FILE" >/dev/null 2>&1; then
    xray_die "Failed to update port/cert in $INBOUND_FILE"
fi

reissue_cert=1
if [ -f "$CERT_FILE" ] || [ -f "$KEY_FILE" ]; then
    case "${XRAY_REISSUE_CERT:-}" in
        1)
            xray_log "Regenerating certificate and key (forced by XRAY_REISSUE_CERT=1)"
            ;;
        0)
            xray_log "Keeping existing certificate and key (XRAY_REISSUE_CERT=0)"
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
                    xray_die "No interactive terminal available. Set XRAY_REISSUE_CERT=1 to regenerate or 0 to keep existing material."
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
    xray_log "Certificate files are missing; generating new ones despite XRAY_REISSUE_CERT=0."
fi

if [ "$reissue_cert" -eq 1 ]; then
    if ! command -v openssl >/dev/null 2>&1; then
        xray_die "openssl binary is required to generate certificates (install package openssl-util)."
    fi

    if [ -n "$XRAY_SERVER_NAME" ]; then
        XRAY_CERT_NAME="$XRAY_SERVER_NAME"
        xray_log "Using XRAY_SERVER_NAME=$XRAY_CERT_NAME from environment"
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
                xray_die "No interactive terminal available. Set XRAY_SERVER_NAME environment variable."
            fi

            if [ -z "$XRAY_CERT_NAME" ] && [ -n "$EXISTING_CERT_CN" ]; then
                XRAY_CERT_NAME="$EXISTING_CERT_CN"
            fi

            if [ -z "$XRAY_CERT_NAME" ]; then
                xray_log "Server name cannot be empty."
            elif ! echo "$XRAY_CERT_NAME" | grep -Eq "^[A-Za-z0-9.-]+$"; then
                xray_log "Server name must contain only letters, digits, dots or hyphens."
                XRAY_CERT_NAME=""
            fi
        done
    fi

    if ! echo "$XRAY_CERT_NAME" | grep -Eq "^[A-Za-z0-9.-]+$"; then
        xray_die "Server name must contain only letters, digits, dots or hyphens."
    fi

    mkdir -p "$CERT_DIR" "$KEY_DIR"

    BACKUP_SUFFIX=$(date +%Y%m%d%H%M%S)
    if [ -f "$CERT_FILE" ]; then
        xray_log "Backing up existing certificate to ${CERT_FILE}.${BACKUP_SUFFIX}.bak"
        mv "$CERT_FILE" "${CERT_FILE}.${BACKUP_SUFFIX}.bak"
    fi
    if [ -f "$KEY_FILE" ]; then
        xray_log "Backing up existing key to ${KEY_FILE}.${BACKUP_SUFFIX}.bak"
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
        xray_die "Failed to generate certificate for $XRAY_CERT_NAME"
    fi
    rm -f "$OPENSSL_CNF"

    chmod 600 "$KEY_FILE"
    chmod 644 "$CERT_FILE"
else
    xray_log "Skipping certificate regeneration; keeping existing files in place."
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
    xray_log "xray-p2p service is listening on port $XRAY_PORT"
elif [ "$port_check_status" -eq 2 ]; then
    xray_log "xray-p2p restarted. Skipping port verification because neither 'ss' nor 'netstat' is available."
    xray_log "Install ip-full (ss) or net-tools-netstat to enable automatic checks."
else
    xray_die "xray-p2p service does not appear to be listening on port $XRAY_PORT"
fi
