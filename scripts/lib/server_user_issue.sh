server_user_issue_usage() {
    cat <<EOF
Usage: $SCRIPT_NAME issue [options] [EMAIL] [SERVER_ADDRESS]
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

server_user_cmd_issue() {
    issue_email_arg=""
    issue_connection_arg=""
    while [ "$#" -gt 0 ]; do
        case "$1" in
            -h|--help)
                server_user_issue_usage 0
                ;;
            --)
                shift
                break
                ;;
            -*)
                printf 'Unknown option: %s\n' "$1" >&2
                server_user_issue_usage 1
                ;;
            *)
                if [ -z "$issue_email_arg" ]; then
                    issue_email_arg="$1"
                elif [ -z "$issue_connection_arg" ]; then
                    issue_connection_arg="$1"
                else
                    printf 'Unexpected argument: %s\n' "$1" >&2
                    server_user_issue_usage 1
                fi
                ;;
        esac
        shift
    done

    while [ "$#" -gt 0 ]; do
        if [ -z "$issue_email_arg" ]; then
            issue_email_arg="$1"
        elif [ -z "$issue_connection_arg" ]; then
            issue_connection_arg="$1"
        else
            printf 'Unexpected argument: %s\n' "$1" >&2
            server_user_issue_usage 1
        fi
        shift
    done

    server_user_require_inbounds
    server_user_init_paths
    xray_check_repo_access 'scripts/server_user.sh issue'

    if [ "${XRAY_SHOW_CLIENTS:-1}" = "1" ]; then
        server_user_show_clients
        printf '\n'
    fi

    mkdir -p "$SERVER_USER_CLIENTS_DIR"
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

    EMAIL="$issue_email_arg"
    if [ -z "$EMAIL" ] && [ -n "${XRAY_CLIENT_EMAIL:-}" ]; then
        EMAIL="$XRAY_CLIENT_EMAIL"
    fi
    AUTO_EMAIL="${XRAY_GENERATED_EMAIL:-$(server_user_generate_client_email)}"

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
    CLIENTS_INPUT="$SERVER_USER_CLIENTS_FILE"
    if [ -f "$SERVER_USER_CLIENTS_FILE" ]; then
        if ! jq empty "$SERVER_USER_CLIENTS_FILE" >/dev/null 2>&1; then
            xray_die "Existing $SERVER_USER_CLIENTS_FILE contains invalid JSON."
        fi
    else
        TMP_BASE_CLIENTS="$(mktemp)"
        printf '[]\n' > "$TMP_BASE_CLIENTS"
        CLIENTS_INPUT="$TMP_BASE_CLIENTS"
    fi

    if jq -e --arg email "$EMAIL" '
        ([ .[]? | select((.email // "") == $email) ] | length) > 0
    ' "$CLIENTS_INPUT" >/dev/null 2>&1; then
        xray_die "Client '$EMAIL' already exists in $SERVER_USER_CLIENTS_FILE"
    fi

    if jq -e --arg email "$EMAIL" '
        ([ .inbounds[]? | .settings.clients[]? | select((.email // "") == $email) ] | length) > 0
    ' "$SERVER_USER_INBOUNDS_FILE" >/dev/null 2>&1; then
        xray_die "Client '$EMAIL' already exists in $SERVER_USER_INBOUNDS_FILE"
    fi

    XRAY_PORT="$(jq -r '
        ( .inbounds // [] )
        | map(select((.protocol // "") == "trojan"))
        | if length == 0 then empty else .[0].port end
    ' "$SERVER_USER_INBOUNDS_FILE")"
    [ -n "$XRAY_PORT" ] && [ "$XRAY_PORT" != "null" ] || xray_die "Unable to determine trojan inbound port from $SERVER_USER_INBOUNDS_FILE"

    CERT_FILE="$(jq -r '
        ( .inbounds // [] )
        | map(select((.protocol // "") == "trojan") | .streamSettings.tlsSettings.certificates[0].certificateFile)
        | map(select(. != null and . != "" and . != "null"))
        | if length == 0 then empty else .[0] end
    ' "$SERVER_USER_INBOUNDS_FILE")"
    [ -n "$CERT_FILE" ] || CERT_FILE="$SERVER_USER_CONFIG_DIR/cert.pem"

    TLS_SNI_HOST="${XRAY_SERVER_NAME:-}"
    [ -n "$TLS_SNI_HOST" ] || TLS_SNI_HOST="${XRAY_DOMAIN:-}"
    [ -n "$TLS_SNI_HOST" ] || TLS_SNI_HOST="${XRAY_CERT_NAME:-}"

    if [ -z "$TLS_SNI_HOST" ] && [ -f "$CERT_FILE" ] && command -v openssl >/dev/null 2>&1; then
        TLS_SNI_HOST="$(openssl x509 -noout -subject -nameopt RFC2253 -in "$CERT_FILE" 2>/dev/null | awk -F'CN=' 'NF>1 {print $2}' | cut -d',' -f1 | sed 's/^ *//;s/ *$//')"
    fi

    TLS_SNI_HOST="$(printf '%s' "$TLS_SNI_HOST" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    if [ -z "$TLS_SNI_HOST" ]; then
        if [ -t 0 ] || [ -r /dev/tty ]; then
            TLS_SNI_HOST="$(server_user_prompt_value 'Enter TLS SNI hostname (domain in certificate)' '')"
        else
            xray_die "TLS SNI hostname not specified. Set XRAY_SERVER_NAME or run interactively."
        fi
    fi

    [ -n "$TLS_SNI_HOST" ] || xray_die "TLS SNI hostname cannot be empty."
    printf '%s' "$TLS_SNI_HOST" | grep -Eq '^[A-Za-z0-9.-]+$' || xray_die "TLS SNI hostname must contain only letters, digits, dots or hyphens."

    CONNECTION_HOST="$issue_connection_arg"
    [ -n "$CONNECTION_HOST" ] || CONNECTION_HOST="${XRAY_SERVER_ADDRESS:-}"
    [ -n "$CONNECTION_HOST" ] || CONNECTION_HOST="${XRAY_SERVER_HOST:-}"

    if [ -z "$CONNECTION_HOST" ]; then
        DEFAULT_CONNECTION_HOST="$TLS_SNI_HOST"
        if [ -t 0 ] || [ -r /dev/tty ]; then
            server_user_run_network_interfaces_helper
            CONNECTION_HOST="$(server_user_prompt_value 'Enter connection address for clients (IP or domain)' "$DEFAULT_CONNECTION_HOST")"
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
    ' "$SERVER_USER_INBOUNDS_FILE" 2>/dev/null)"
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

    CLIENT_ID="$(server_user_generate_uuid)"
    PASSWORD="$(server_user_generate_password)"
    ISSUED_AT="$(date -u '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || date '+%Y-%m-%dT%H:%M:%SZ')"
    ISSUED_BY_DEFAULT="$(id -un 2>/dev/null || echo unknown)"
    ISSUED_BY="${XRAY_ISSUED_BY:-$ISSUED_BY_DEFAULT}"
    CLIENT_LABEL="$(printf '%s' "$EMAIL" | jq -Rr @uri)"
    LINK_HOST="$(server_user_format_link_host "$CONNECTION_HOST")"
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

    mv "$TMP_CLIENTS" "$SERVER_USER_CLIENTS_FILE"
    chmod 600 "$SERVER_USER_CLIENTS_FILE"
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
      ))' "$SERVER_USER_INBOUNDS_FILE" > "$TMP_INBOUNDS"

    mv "$TMP_INBOUNDS" "$SERVER_USER_INBOUNDS_FILE"
    chmod 644 "$SERVER_USER_INBOUNDS_FILE"
    TMP_INBOUNDS=""

    xray_restart_service "xray-p2p" "/etc/init.d/xray-p2p"

    xray_log "Client '$EMAIL' issued with id $CLIENT_ID."
    xray_log "Configuration files updated: $SERVER_USER_CLIENTS_FILE, $SERVER_USER_INBOUNDS_FILE"

    printf '%s\n' "$CLIENT_LINK"
}