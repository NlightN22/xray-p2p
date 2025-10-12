#!/bin/sh
set -eu

SCRIPT_NAME=${0##*/}

usage() {
    cat <<EOF
Usage: $SCRIPT_NAME [options] [EMAIL] [SERVER_ADDRESS]

Create a new XRAY client and emit the connection details.

Options:
  -h, --help        Show this help message and exit.

Arguments:
  EMAIL            Optional client identifier; auto-generated when omitted.
  SERVER_ADDRESS   Optional connection host; overrides env/prompt.

Environment variables:
  XRAY_CLIENT_EMAIL   Preseed client email when no positional argument is given.
  XRAY_AUTO_EMAIL     Set to 1 to accept an auto-generated client email.
  XRAY_SERVER_NAME    TLS SNI hostname fallback when certificates lack CN.
  XRAY_SERVER_ADDRESS Connection address advertised to clients.
EOF
    exit "${1:-0}"
}

email_arg=""
connection_arg=""

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
            printf 'Unknown option: %s\n' "$1" >&2
            usage 1
            ;;
        *)
            if [ -z "$email_arg" ]; then
                email_arg="$1"
            elif [ -z "$connection_arg" ]; then
                connection_arg="$1"
            else
                printf 'Unexpected argument: %s\n' "$1" >&2
                usage 1
            fi
            ;;
    esac
    shift
done

while [ "$#" -gt 0 ]; do
    if [ -z "$email_arg" ]; then
        email_arg="$1"
    elif [ -z "$connection_arg" ]; then
        connection_arg="$1"
    else
        printf 'Unexpected argument: %s\n' "$1" >&2
        usage 1
    fi
    shift
done

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

umask 077

XRAY_COMMON_LIB_PATH_DEFAULT="lib/common.sh"
XRAY_COMMON_LIB_REMOTE_PATH_DEFAULT="scripts/lib/common.sh"

COMMON_LIB_REMOTE_PATH="${XRAY_COMMON_LIB_REMOTE_PATH:-$XRAY_COMMON_LIB_REMOTE_PATH_DEFAULT}"
COMMON_LIB_LOCAL_PATH="${XRAY_COMMON_LIB_PATH:-$XRAY_COMMON_LIB_PATH_DEFAULT}"

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
            xray_die "No interactive terminal available. Set required environment variables."
        fi

        if [ -z "$value" ] && [ -n "$default_value" ]; then
            value="$default_value"
        fi

        if [ -z "$value" ]; then
            xray_log "Value cannot be empty."
        fi
    done

    printf '%s' "$value"
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

    xray_die "Unable to generate UUID; install uuidgen or ensure /proc/sys/kernel/random/uuid is available."
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

    xray_die "Unable to generate password; install openssl or ensure /dev/urandom is available."
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

run_network_interfaces_helper() {
    if [ "${XRAY_SHOW_INTERFACES:-1}" != "1" ]; then
        return
    fi

    helper_local="${XRAY_INTERFACES_SCRIPT_PATH:-lib/network_interfaces.sh}"
    helper_remote="${XRAY_INTERFACES_REMOTE_PATH:-scripts/lib/network_interfaces.sh}"

    if xray_run_repo_script optional "$helper_local" "$helper_remote" >&2; then
        printf '\n' >&2
    fi
}

CONFIG_DIR="${XRAY_CONFIG_DIR:-/etc/xray-p2p}"
INBOUNDS_FILE="${XRAY_INBOUNDS_FILE:-$CONFIG_DIR/inbounds.json}"
CLIENTS_DIR="${XRAY_CLIENTS_DIR:-$CONFIG_DIR/config}"
CLIENTS_FILE="${XRAY_CLIENTS_FILE:-$CLIENTS_DIR/clients.json}"

[ -f "$INBOUNDS_FILE" ] || xray_die "Inbound configuration not found: $INBOUNDS_FILE"

xray_require_cmd jq

xray_check_repo_access 'scripts/user_issue.sh'

if [ "${XRAY_SHOW_CLIENTS:-1}" = "1" ]; then
    if clients_output=$(XRAY_CONFIG_DIR="$CONFIG_DIR" \
            XRAY_INBOUNDS_FILE="$INBOUNDS_FILE" \
            XRAY_CLIENTS_FILE="$CLIENTS_FILE" \
            xray_run_repo_script optional "lib/user_list.sh" "scripts/lib/user_list.sh" 2>&1); then
        if [ -n "$clients_output" ]; then
            xray_log "Current clients (email password status):"
            printf '%s\n' "$clients_output"
        fi
    elif [ -n "$clients_output" ]; then
        printf '%s\n' "$clients_output" >&2
    fi
    printf '\n'
fi

mkdir -p "$CLIENTS_DIR"

TMP_CLIENTS=""
TMP_INBOUNDS=""
TMP_BASE_CLIENTS=""

cleanup() {
    if [ -n "$TMP_CLIENTS" ] && [ -f "$TMP_CLIENTS" ]; then
        rm -f "$TMP_CLIENTS"
    fi
    if [ -n "$TMP_INBOUNDS" ] && [ -f "$TMP_INBOUNDS" ]; then
        rm -f "$TMP_INBOUNDS"
    fi
    if [ -n "$TMP_BASE_CLIENTS" ] && [ -f "$TMP_BASE_CLIENTS" ]; then
        rm -f "$TMP_BASE_CLIENTS"
    fi
}
trap cleanup EXIT INT TERM

EMAIL="$email_arg"
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

[ -n "$EMAIL" ] || xray_die "Client email cannot be empty."
printf '%s' "$EMAIL" | grep -Eq '^[^[:space:]]+$' || xray_die "Client email must not contain whitespace."
CLIENTS_INPUT="$CLIENTS_FILE"
if [ -f "$CLIENTS_FILE" ]; then
    if ! jq empty "$CLIENTS_FILE" >/dev/null 2>&1; then
        xray_die "Existing $CLIENTS_FILE contains invalid JSON."
    fi
else
    TMP_BASE_CLIENTS="$(mktemp)"
    printf '[]\n' > "$TMP_BASE_CLIENTS"
    CLIENTS_INPUT="$TMP_BASE_CLIENTS"
fi

if jq -e --arg email "$EMAIL" '
    ([ .[]? | select((.email // "") == $email) ] | length) > 0
' "$CLIENTS_INPUT" >/dev/null 2>&1; then
    xray_die "Client '$EMAIL' already exists in $CLIENTS_FILE"
fi

if jq -e --arg email "$EMAIL" '
    ([ .inbounds[]? | .settings.clients[]? | select((.email // "") == $email) ] | length) > 0
' "$INBOUNDS_FILE" >/dev/null 2>&1; then
    xray_die "Client '$EMAIL' already exists in $INBOUNDS_FILE"
fi

XRAY_PORT="$(jq -r '
    ( .inbounds // [] )
    | map(select((.protocol // "") == "trojan"))
    | if length == 0 then empty else .[0].port end
' "$INBOUNDS_FILE")"

[ -n "$XRAY_PORT" ] && [ "$XRAY_PORT" != "null" ] || xray_die "Unable to determine trojan inbound port from $INBOUNDS_FILE"

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
        xray_die "TLS SNI hostname not specified. Set XRAY_SERVER_NAME or run interactively."
    fi
fi

[ -n "$TLS_SNI_HOST" ] || xray_die "TLS SNI hostname cannot be empty."
printf '%s' "$TLS_SNI_HOST" | grep -Eq '^[A-Za-z0-9.-]+$' || xray_die "TLS SNI hostname must contain only letters, digits, dots or hyphens."

CONNECTION_HOST="$connection_arg"
[ -n "$CONNECTION_HOST" ] || CONNECTION_HOST="${XRAY_SERVER_ADDRESS:-}"
[ -n "$CONNECTION_HOST" ] || CONNECTION_HOST="${XRAY_SERVER_HOST:-}"

if [ -z "$CONNECTION_HOST" ]; then
    DEFAULT_CONNECTION_HOST="$TLS_SNI_HOST"
    if [ -t 0 ] || [ -r /dev/tty ]; then
        run_network_interfaces_helper
        CONNECTION_HOST="$(prompt_value 'Enter connection address for clients (IP or domain)' "$DEFAULT_CONNECTION_HOST")"
    else
        CONNECTION_HOST="$DEFAULT_CONNECTION_HOST"
    fi
fi

CONNECTION_HOST="$(printf '%s' "$CONNECTION_HOST" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
[ -n "$CONNECTION_HOST" ] || xray_die "Connection address cannot be empty."
printf '%s' "$CONNECTION_HOST" | grep -Eq '^[^[:space:]]+$' || xray_die "Connection address must not contain whitespace."

TLS_ALLOW_INSECURE="$(jq -r '
    def truthy:
        if . == null then false
        elif type == "boolean" then .
        elif type == "number" then . != 0
        elif type == "string" then
            (gsub("[[:space:]]"; "") | ascii_downcase) as $s
            | ($s == "1" or $s == "true" or $s == "yes" or $s == "on")
        else false end;

    ( .inbounds // [] )
    | map(select((.protocol // "") == "trojan"))
    | map(
        ((.streamSettings // {}) | (.tlsSettings // {})) as $tls
        | [
            $tls.allowInsecure,
            (($tls.certificates // [])[]? | .allowInsecure)
        ]
        | map(truthy)
        | any
    )
    | any
    | if . then "1" else "0" end
' "$INBOUNDS_FILE" 2>/dev/null)"
TLS_ALLOW_INSECURE="$(printf '%s' "$TLS_ALLOW_INSECURE" | tr -d '[:space:]' | tr '[:upper:]' '[:lower:]')"
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
TLS_ALLOW_INSECURE="$(printf '%s' "$TLS_ALLOW_INSECURE" | tr -d '[:space:]' | tr '[:upper:]' '[:lower:]')"
case "$TLS_ALLOW_INSECURE" in
    1|true|yes|on)
        TLS_ALLOW_INSECURE=1
        ;;
    0|false|no|off|'')
        TLS_ALLOW_INSECURE=0
        ;;
    *)
        xray_log "Unrecognised XRAY_ALLOW_INSECURE value '$TLS_ALLOW_INSECURE'; defaulting to 0"
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

xray_restart_service "xray-p2p" "/etc/init.d/xray-p2p"

xray_log "Client '$EMAIL' issued with id $CLIENT_ID."
xray_log "Configuration files updated: $CLIENTS_FILE, $INBOUNDS_FILE"

printf '%s\n' "$CLIENT_LINK"
