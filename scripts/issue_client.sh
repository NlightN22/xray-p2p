#!/bin/sh
set -eu

umask 077

log() {
    printf '%s\n' "$*" >&2
}

die() {
    printf 'Error: %s\n' "$*" >&2
    exit 1
}

prompt_value() {
    prompt="$1"
    default_value="$2"
    value=""
    while [ -z "$value" ]; do
        if [ -n "$default_value" ]; then
            printf '%s [%s]: ' "$prompt" "$default_value" >&2
        else
            printf '%s: ' "$prompt" >&2
        fi

        if [ -t 0 ]; then
            IFS= read -r value
        elif [ -r /dev/tty ]; then
            IFS= read -r value </dev/tty
        else
            die "No interactive terminal available. Set required environment variables."
        fi

        if [ -z "$value" ] && [ -n "$default_value" ]; then
            value="$default_value"
        fi

        if [ -z "$value" ]; then
            log "Value cannot be empty."
        fi
    done

    printf '%s' "$value"
}

require_cmd() {
    cmd="$1"
    if ! command -v "$cmd" >/dev/null 2>&1; then
        case "$cmd" in
            jq)
                die "Required command 'jq' not found. Install it before running this script. For OpenWrt run: opkg update && opkg install jq"
                ;;
            *)
                die "Required command '$cmd' not found. Install it before running this script."
                ;;
        esac
    fi
}
generate_uuid() {
    if command -v uuidgen >/dev/null 2>&1; then
        uuidgen | tr '[:upper:]' '[:lower:]'
        return
    fi

    if [ -r /proc/sys/kernel/random/uuid ]; then
        tr '[:upper:]' '[:lower:]' < /proc/sys/kernel/random/uuid
        return
    fi

    if command -v openssl >/dev/null 2>&1; then
        openssl rand -hex 16 | sed -E 's/(.{8})(.{4})(.{4})(.{4})(.{12})/\1-\2-\3-\4-\5/'
        return
    fi

    if command -v hexdump >/dev/null 2>&1; then
        hexdump -n 16 -v -e '/1 "%02x"' /dev/urandom 2>/dev/null | sed -E 's/(.{8})(.{4})(.{4})(.{4})(.{12})/\1-\2-\3-\4-\5/'
        return
    fi

    die "Unable to generate UUID; install uuidgen or ensure /proc/sys/kernel/random/uuid is available."
}

generate_password() {
    if command -v openssl >/dev/null 2>&1; then
        openssl rand -hex 16
        return
    fi

    if command -v hexdump >/dev/null 2>&1; then
        hexdump -n 16 -v -e '/1 "%02x"' /dev/urandom 2>/dev/null
        return
    fi

    if [ -r /proc/sys/kernel/random/uuid ]; then
        tr -d '-' < /proc/sys/kernel/random/uuid | cut -c1-32
        return
    fi

    die "Unable to generate password; install openssl or ensure /dev/urandom is available."
}

generate_client_email() {
    prefix="${XRAY_EMAIL_PREFIX:-client}"
    domain="${XRAY_EMAIL_DOMAIN:-auto.local}"
    uuid_comp="$(generate_uuid | tr -d '-' | cut -c1-12)"
    # Fall back to timestamp if UUID generation failed for any reason
    if [ -z "$uuid_comp" ]; then
        uuid_comp="$(date -u '+%Y%m%d%H%M%S' 2>/dev/null || date '+%Y%m%d%H%M%S')"
    fi
    printf '%s-%s@%s' "$prefix" "$uuid_comp" "$domain"
}

format_link_host() {
    host="$1"
    if printf '%s' "$host" | grep -q ':'; then
        case "$host" in
            [[]*[]])
                printf '%s' "$host"
                ;;
            *)
                printf '[%s]' "$host"
                ;;
        esac
    else
        printf '%s' "$host"
    fi
}

check_repo_access() {
    [ "${XRAY_SKIP_REPO_CHECK:-0}" = "1" ] && return

    base_url="${XRAY_REPO_BASE_URL:-https://raw.githubusercontent.com/NlightN22/xray-p2p/main}"
    check_path="${XRAY_REPO_CHECK_PATH:-scripts/issue_client.sh}"
    timeout="${XRAY_REPO_CHECK_TIMEOUT:-5}"

    case "$base_url" in
        */) repo_url="${base_url}${check_path}" ;;
        *) repo_url="${base_url}/${check_path}" ;;
    esac

    last_tool=""
    for tool in curl wget; do
        case "$tool" in
            curl)
                command -v curl >/dev/null 2>&1 || continue
                if curl -fsSL --max-time "$timeout" "$repo_url" >/dev/null 2>&1; then
                    return
                fi
                last_tool="curl"
                ;;
            wget)
                command -v wget >/dev/null 2>&1 || continue
                if wget -q -T "$timeout" -O /dev/null "$repo_url"; then
                    return
                fi
                last_tool="wget"
                ;;
        esac
    done

    if [ -z "$last_tool" ]; then
        log "Neither curl nor wget is available to verify repository accessibility; skipping check."
        return
    fi

    die "Unable to access repository resource $repo_url (last attempt via $last_tool). Set XRAY_SKIP_REPO_CHECK=1 to bypass."
}

show_existing_clients() {
    if [ "${XRAY_SHOW_CLIENTS:-1}" != "1" ]; then
        return
    fi

    log "Current clients (email password status):"

    base_url="${XRAY_REPO_BASE_URL:-https://raw.githubusercontent.com/NlightN22/xray-p2p/main}"
    list_path="${XRAY_LIST_SCRIPT_PATH:-scripts/list_clients.sh}"
    base_trimmed="${base_url%/}"
    case "$list_path" in
        /*) list_url="${base_trimmed}${list_path}" ;;
        *) list_url="${base_trimmed}/${list_path}" ;;
    esac

    tmp_list="$(mktemp 2>/dev/null)" || tmp_list=""
    if [ -z "$tmp_list" ]; then
        log "Unable to create temporary file for list script"
        printf '\n'
        return
    fi

    if curl -fsSL "$list_url" -o "$tmp_list"; then
        if ! env \
            XRAY_CONFIG_DIR="$CONFIG_DIR" \
            XRAY_INBOUNDS_FILE="$INBOUNDS_FILE" \
            XRAY_CLIENTS_FILE="$CLIENTS_FILE" \
            XRAY_SKIP_REPO_CHECK=1 \
            sh "$tmp_list"; then
            log "List script returned an error"
        fi
    else
        log "Failed to download $list_url"
    fi
    rm -f "$tmp_list"
    printf '\n'
}

backup_file() {
    file_path="$1"
    [ -f "$file_path" ] || return 0

    stamp="$(date +%Y%m%d%H%M%S)"
    backup_path="${file_path}.${stamp}.bak"

    if cp "$file_path" "$backup_path"; then
        log "Backup created: $backup_path"
    else
        die "Failed to create backup for $file_path"
    fi
}

CONFIG_DIR="${XRAY_CONFIG_DIR:-/etc/xray}"
INBOUNDS_FILE="${XRAY_INBOUNDS_FILE:-$CONFIG_DIR/inbounds.json}"
CLIENTS_DIR="${XRAY_CLIENTS_DIR:-$CONFIG_DIR/config}"
CLIENTS_FILE="${XRAY_CLIENTS_FILE:-$CLIENTS_DIR/clients.json}"
SERVICE_NAME="${XRAY_SERVICE_NAME:-xray}"

[ -f "$INBOUNDS_FILE" ] || die "Inbound configuration not found: $INBOUNDS_FILE"

require_cmd jq
require_cmd curl

check_repo_access

show_existing_clients

mkdir -p "$CLIENTS_DIR"

TMP_CLIENTS=""
TMP_INBOUNDS=""
TMP_BASE_CLIENTS=""

cleanup() {
    [ -n "$TMP_CLIENTS" ] && [ -f "$TMP_CLIENTS" ] && rm -f "$TMP_CLIENTS"
    [ -n "$TMP_INBOUNDS" ] && [ -f "$TMP_INBOUNDS" ] && rm -f "$TMP_INBOUNDS"
    [ -n "$TMP_BASE_CLIENTS" ] && [ -f "$TMP_BASE_CLIENTS" ] && rm -f "$TMP_BASE_CLIENTS"
}
trap cleanup EXIT INT TERM

EMAIL="${1:-}"
if [ -z "$EMAIL" ] && [ -n "${XRAY_CLIENT_EMAIL:-}" ]; then
    EMAIL="$XRAY_CLIENT_EMAIL"
fi

AUTO_EMAIL="${XRAY_GENERATED_EMAIL:-$(generate_client_email)}"

if [ -z "$EMAIL" ]; then
    if [ "${XRAY_AUTO_EMAIL:-0}" = "1" ]; then
        EMAIL="$AUTO_EMAIL"
    else
        if [ -t 0 ] || [ -r /dev/tty ]; then
            printf 'Enter client email (identifier, leave empty to auto-generate): ' >&2
            if [ -t 0 ]; then
                IFS= read -r EMAIL
            else
                IFS= read -r EMAIL </dev/tty
            fi
            [ -n "$EMAIL" ] || EMAIL="$AUTO_EMAIL"
        else
            EMAIL="$AUTO_EMAIL"
        fi
    fi
fi

EMAIL="$(printf '%s' "$EMAIL" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"

[ -n "$EMAIL" ] || die "Client email cannot be empty."
printf '%s' "$EMAIL" | grep -Eq '^[^[:space:]]+$' || die "Client email must not contain whitespace."
CLIENTS_INPUT="$CLIENTS_FILE"
if [ -f "$CLIENTS_FILE" ]; then
    if ! jq empty "$CLIENTS_FILE" >/dev/null 2>&1; then
        die "Existing $CLIENTS_FILE contains invalid JSON."
    fi
else
    TMP_BASE_CLIENTS="$(mktemp)"
    printf '[]\n' > "$TMP_BASE_CLIENTS"
    CLIENTS_INPUT="$TMP_BASE_CLIENTS"
fi

if jq -e --arg email "$EMAIL" '
    ([ .[]? | select((.email // "") == $email) ] | length) > 0
' "$CLIENTS_INPUT" >/dev/null 2>&1; then
    die "Client '$EMAIL' already exists in $CLIENTS_FILE"
fi

if jq -e --arg email "$EMAIL" '
    ([ .inbounds[]? | .settings.clients[]? | select((.email // "") == $email) ] | length) > 0
' "$INBOUNDS_FILE" >/dev/null 2>&1; then
    die "Client '$EMAIL' already exists in $INBOUNDS_FILE"
fi

XRAY_PORT="$(jq -r '
    ( .inbounds // [] )
    | map(select((.protocol // "") == "trojan"))
    | if length == 0 then empty else .[0].port end
' "$INBOUNDS_FILE")"

[ -n "$XRAY_PORT" ] && [ "$XRAY_PORT" != "null" ] || die "Unable to determine trojan inbound port from $INBOUNDS_FILE"

CERT_FILE="$(jq -r '
    ( .inbounds // [] )
    | map(select((.protocol // "") == "trojan") | .streamSettings.tlsSettings.certificates[0].certificateFile)
    | map(select(. != null and . != "" and . != "null"))
    | if length == 0 then empty else .[0] end
' "$INBOUNDS_FILE")"

[ -n "$CERT_FILE" ] || CERT_FILE="$CONFIG_DIR/cert.pem"

TLS_SNI_HOST="${XRAY_SERVER_NAME:-}"
[ -n "$TLS_SNI_HOST" ] || TLS_SNI_HOST="${XRAY_DOMAIN:-}"
[ -n "$TLS_SNI_HOST" ] || TLS_SNI_HOST="${XRAY_CERT_NAME:-}"

if [ -z "$TLS_SNI_HOST" ] && [ -f "$CERT_FILE" ] && command -v openssl >/dev/null 2>&1; then
    TLS_SNI_HOST="$(openssl x509 -noout -subject -nameopt RFC2253 -in "$CERT_FILE" 2>/dev/null | awk -F'CN=' 'NF>1 {print $2}' | cut -d',' -f1 | sed 's/^ *//;s/ *$//')"
fi

TLS_SNI_HOST="$(printf '%s' "$TLS_SNI_HOST" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"

if [ -z "$TLS_SNI_HOST" ]; then
    if [ -t 0 ] || [ -r /dev/tty ]; then
        TLS_SNI_HOST="$(prompt_value 'Enter TLS SNI hostname (domain in certificate)' '')"
    else
        die "TLS SNI hostname not specified. Set XRAY_SERVER_NAME or run interactively."
    fi
fi

[ -n "$TLS_SNI_HOST" ] || die "TLS SNI hostname cannot be empty."
printf '%s' "$TLS_SNI_HOST" | grep -Eq '^[A-Za-z0-9.-]+$' || die "TLS SNI hostname must contain only letters, digits, dots or hyphens."

CONNECTION_HOST="${XRAY_SERVER_ADDRESS:-}"
[ -n "$CONNECTION_HOST" ] || CONNECTION_HOST="${XRAY_SERVER_HOST:-}"

if [ -z "$CONNECTION_HOST" ]; then
    DEFAULT_CONNECTION_HOST="$TLS_SNI_HOST"
    if [ -t 0 ] || [ -r /dev/tty ]; then
        CONNECTION_HOST="$(prompt_value 'Enter connection address for clients (IP or domain)' "$DEFAULT_CONNECTION_HOST")"
    else
        CONNECTION_HOST="$DEFAULT_CONNECTION_HOST"
    fi
fi

CONNECTION_HOST="$(printf '%s' "$CONNECTION_HOST" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
[ -n "$CONNECTION_HOST" ] || die "Connection address cannot be empty."
printf '%s' "$CONNECTION_HOST" | grep -Eq '^[^[:space:]]+$' || die "Connection address must not contain whitespace."

TLS_ALLOW_INSECURE="$(jq -r '
    ( .inbounds // [] )
    | map(select((.protocol // "") == "trojan") | .streamSettings.tlsSettings.allowInsecure)
    | map(select(. != null))
    | if length == 0 then empty else .[0] end
' "$INBOUNDS_FILE" 2>/dev/null)"
TLS_ALLOW_INSECURE="$(printf '%s' "$TLS_ALLOW_INSECURE" | tr '[:upper:]' '[:lower:]')"
case "$TLS_ALLOW_INSECURE" in
    1|true|yes|on)
        TLS_ALLOW_INSECURE=1
        ;;
    *)
        TLS_ALLOW_INSECURE=0
        ;;
esac

if [ -n "${XRAY_ALLOW_INSECURE:-}" ]; then
    TLS_ALLOW_INSECURE="${XRAY_ALLOW_INSECURE}"
fi
TLS_ALLOW_INSECURE="$(printf '%s' "$TLS_ALLOW_INSECURE" | tr '[:upper:]' '[:lower:]')"
case "$TLS_ALLOW_INSECURE" in
    1|true|yes|on)
        TLS_ALLOW_INSECURE=1
        ;;
    0|false|no|off|'')
        TLS_ALLOW_INSECURE=0
        ;;
    *)
        log "Unrecognised XRAY_ALLOW_INSECURE value '$TLS_ALLOW_INSECURE'; defaulting to 0"
        TLS_ALLOW_INSECURE=0
        ;;
esac

CLIENT_ID="$(generate_uuid)"
PASSWORD="$(generate_password)"
ISSUED_AT="$(date -u '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || date '+%Y-%m-%dT%H:%M:%SZ')"
ISSUED_BY_DEFAULT="$(id -un 2>/dev/null || echo unknown)"
ISSUED_BY="${XRAY_ISSUED_BY:-$ISSUED_BY_DEFAULT}"
CLIENT_LABEL="$(printf '%s' "$EMAIL" | jq -Rr @uri)"
LINK_HOST="$(format_link_host "$CONNECTION_HOST")"
TLS_QUERY="security=tls&type=tcp"
if [ "$TLS_ALLOW_INSECURE" = 1 ]; then
    TLS_QUERY="${TLS_QUERY}&allowInsecure=1"
fi
TLS_QUERY="${TLS_QUERY}&sni=${TLS_SNI_HOST}"
CLIENT_LINK="trojan://${PASSWORD}@${LINK_HOST}:${XRAY_PORT}?${TLS_QUERY}#${CLIENT_LABEL}"

if [ "${XRAY_SKIP_BACKUP:-0}" != "1" ]; then
    [ -f "$CLIENTS_FILE" ] && backup_file "$CLIENTS_FILE"
    backup_file "$INBOUNDS_FILE"
fi

TMP_CLIENTS="$(mktemp)"
jq   --arg id "$CLIENT_ID"   --arg password "$PASSWORD"   --arg email "$EMAIL"   --arg status "issued"   --arg issued_at "$ISSUED_AT"   --arg issued_by "$ISSUED_BY"   --arg link "$CLIENT_LINK"   '(. // []) + [{
      id: $id,
      password: $password,
      email: $email,
      status: $status,
      issued_at: $issued_at,
      issued_by: $issued_by,
      activated_at: null,
      activated_from: null,
      link: $link,
      notes: ""
  }]' "$CLIENTS_INPUT" > "$TMP_CLIENTS"

mv "$TMP_CLIENTS" "$CLIENTS_FILE"
chmod 600 "$CLIENTS_FILE"
TMP_CLIENTS=""

TMP_INBOUNDS="$(mktemp)"
jq   --arg email "$EMAIL"   --arg password "$PASSWORD"   --arg port "$XRAY_PORT"   '.inbounds |= ( ( . // [] ) | map(
      if (.protocol // "") == "trojan" and (.port|tostring) == $port then
          .settings = (.settings // {})
          | .settings.clients = ((.settings.clients // []) + [{
              password: $password,
              email: $email
          }])
      else
          .
      end
  ))' "$INBOUNDS_FILE" > "$TMP_INBOUNDS"

mv "$TMP_INBOUNDS" "$INBOUNDS_FILE"
chmod 644 "$INBOUNDS_FILE"
TMP_INBOUNDS=""

if [ "${XRAY_SKIP_RESTART:-0}" = "1" ]; then
    log "Skipping ${SERVICE_NAME} restart (XRAY_SKIP_RESTART=1)."
else
    SERVICE_SCRIPT="/etc/init.d/$SERVICE_NAME"
    [ -x "$SERVICE_SCRIPT" ] || die "Service script not found or not executable: $SERVICE_SCRIPT"
    log "Restarting $SERVICE_NAME service"
    "$SERVICE_SCRIPT" restart || die "Failed to restart $SERVICE_NAME service."
fi

log "Client '$EMAIL' issued with id $CLIENT_ID."
log "Configuration files updated: $CLIENTS_FILE, $INBOUNDS_FILE"

printf '%s\n' "$CLIENT_LINK"

