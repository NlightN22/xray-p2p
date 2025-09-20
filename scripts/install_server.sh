#!/bin/sh
# Install XRAY server
curl -s https://gist.githubusercontent.com/NlightN22/d410a3f9dd674308999f13f3aeb558ff/raw/da2634081050deefd504504d5ecb86406381e366/install_xray_openwrt.sh | sh

XRAY_CONFIG_DIR="/etc/xray"
if [ ! -d "$XRAY_CONFIG_DIR" ]; then
    echo "Creating XRAY configuration directory at $XRAY_CONFIG_DIR"
    mkdir -p "$XRAY_CONFIG_DIR"
fi

CONFIG_BASE_URL="https://raw.githubusercontent.com/NlightN22/xray-p2p/main/config_templates/server"
CONFIG_FILES="inbounds.json logs.json outbounds.json"
for file in $CONFIG_FILES; do
    echo "Downloading $file to $XRAY_CONFIG_DIR"
    if ! curl -fsSL "$CONFIG_BASE_URL/$file" -o "$XRAY_CONFIG_DIR/$file"; then
        echo "Failed to download $file" >&2
        exit 1
    fi
    chmod 644 "$XRAY_CONFIG_DIR/$file"
done

INBOUND_FILE="$XRAY_CONFIG_DIR/inbounds.json"
if [ ! -f "$INBOUND_FILE" ]; then
    echo "Inbound configuration $INBOUND_FILE is missing" >&2
    exit 1
fi

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

if ! command -v openssl >/dev/null 2>&1; then
    append_missing "- openssl (install with: opkg update && opkg install openssl-util)"
fi

if [ -n "$missing_deps" ]; then
    echo "Missing required dependencies before continuing:" >&2
    printf '%b\n' "$missing_deps" >&2
    exit 1
fi

XRAY_CONF_DIR_UCI="$(uci -q get xray.config.confdir 2>/dev/null)"
if [ -z "$XRAY_CONF_DIR_UCI" ]; then
    echo "Unable to read xray.config.confdir via uci" >&2
    exit 1
fi

if [ "$XRAY_CONF_DIR_UCI" != "$XRAY_CONFIG_DIR" ]; then
    echo "UCI confdir ($XRAY_CONF_DIR_UCI) does not match expected path $XRAY_CONFIG_DIR" >&2
    echo "Update it with: uci set xray.config.confdir='$XRAY_CONFIG_DIR'; uci commit xray" >&2
    exit 1
fi

if [ "$(uci -q get xray.enabled.enabled 2>/dev/null)" != "1" ]; then
    echo "Enabling xray service to start on boot"
    uci set xray.enabled.enabled='1'
    uci commit xray
fi

DEFAULT_PORT=8443

if [ -n "$XRAY_PORT" ]; then
    echo "Using XRAY_PORT=$XRAY_PORT from environment"
else
    printf "Enter external port for XRAY [%s]: " "$DEFAULT_PORT"
    if [ -t 0 ]; then
        IFS= read -r XRAY_PORT
    elif [ -r /dev/tty ]; then
        IFS= read -r XRAY_PORT </dev/tty
    else
        echo "No interactive terminal available. Set XRAY_PORT environment variable." >&2
        exit 1
    fi
fi

if [ -z "$XRAY_PORT" ]; then
    XRAY_PORT="$DEFAULT_PORT"
fi

if ! echo "$XRAY_PORT" | grep -Eq "^[0-9]+$"; then
    echo "Port must be numeric" >&2
    exit 1
fi

if [ "$XRAY_PORT" -le 0 ] || [ "$XRAY_PORT" -gt 65535 ]; then
    echo "Port must be between 1 and 65535" >&2
    exit 1
fi

if [ -n "$XRAY_SERVER_NAME" ]; then
    XRAY_CERT_NAME="$XRAY_SERVER_NAME"
    echo "Using XRAY_SERVER_NAME=$XRAY_CERT_NAME from environment"
else
    XRAY_CERT_NAME=""
    while [ -z "$XRAY_CERT_NAME" ]; do
        printf "Enter server name for TLS certificate (e.g. vpn.example.com): "
        if [ -t 0 ]; then
            IFS= read -r XRAY_CERT_NAME
        elif [ -r /dev/tty ]; then
            IFS= read -r XRAY_CERT_NAME </dev/tty
        else
            echo "No interactive terminal available. Set XRAY_SERVER_NAME environment variable." >&2
            exit 1
        fi

        if [ -z "$XRAY_CERT_NAME" ]; then
            echo "Server name cannot be empty." >&2
        elif ! echo "$XRAY_CERT_NAME" | grep -Eq "^[A-Za-z0-9.-]+$"; then
            echo "Server name must contain only letters, digits, dots or hyphens." >&2
            XRAY_CERT_NAME=""
        fi
    done
fi

if ! echo "$XRAY_CERT_NAME" | grep -Eq "^[A-Za-z0-9.-]+$"; then
    echo "Server name must contain only letters, digits, dots or hyphens." >&2
    exit 1
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
    echo "Failed to update port in $INBOUND_FILE" >&2
    exit 1
fi

CERT_FILE=$(awk -F'"' '/"certificateFile"/ {print $4; exit}' "$INBOUND_FILE")
KEY_FILE=$(awk -F'"' '/"keyFile"/ {print $4; exit}' "$INBOUND_FILE")

[ -z "$CERT_FILE" ] && CERT_FILE="$XRAY_CONFIG_DIR/cert.pem"
[ -z "$KEY_FILE" ] && KEY_FILE="$XRAY_CONFIG_DIR/key.pem"

CERT_DIR=$(dirname "$CERT_FILE")
KEY_DIR=$(dirname "$KEY_FILE")
mkdir -p "$CERT_DIR" "$KEY_DIR"

BACKUP_SUFFIX=$(date +%Y%m%d%H%M%S)
if [ -f "$CERT_FILE" ]; then
    echo "Backing up existing certificate to ${CERT_FILE}.${BACKUP_SUFFIX}.bak"
    mv "$CERT_FILE" "${CERT_FILE}.${BACKUP_SUFFIX}.bak"
fi
if [ -f "$KEY_FILE" ]; then
    echo "Backing up existing key to ${KEY_FILE}.${BACKUP_SUFFIX}.bak"
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
    echo "Failed to generate certificate for $XRAY_CERT_NAME" >&2
    exit 1
fi
rm -f "$OPENSSL_CNF"

chmod 600 "$KEY_FILE"
chmod 644 "$CERT_FILE"

echo "Restarting xray service"
if ! /etc/init.d/xray restart; then
    echo "Failed to restart xray service" >&2
    exit 1
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
    echo "XRAY service is listening on port $XRAY_PORT"
elif [ "$port_check_status" -eq 2 ]; then
    echo "XRAY service restarted. Skipping port verification because neither 'ss' nor 'netstat' is available."
    echo "Install ip-full (ss) or net-tools-netstat to enable automatic checks."
else
    echo "XRAY service does not appear to be listening on port $XRAY_PORT" >&2
    exit 1
fi
