server_user_remove_usage() {
    cat <<EOF
Usage: $SCRIPT_NAME remove [options] [EMAIL]

Remove an XRAY client from both the registry and inbound configuration.

Options:
  -h, --help        Show this help message and exit.

Arguments:
  EMAIL            Optional client identifier; prompts when omitted.

Environment variables:
  XRAY_CLIENT_EMAIL   Client email fallback when no argument is provided.
EOF
    exit "${1:-0}"
}

server_user_cmd_remove() {
    remove_email_arg=""

    while [ "$#" -gt 0 ]; do
        case "$1" in
            -h|--help)
                server_user_remove_usage 0
                ;;
            --)
                shift
                break
                ;;
            -*)
                printf 'Unknown option: %s\n' "$1" >&2
                server_user_remove_usage 1
                ;;
            *)
                if [ -n "$remove_email_arg" ]; then
                    printf 'Unexpected argument: %s\n' "$1" >&2
                    server_user_remove_usage 1
                fi
                remove_email_arg="$1"
                ;;
        esac
        shift
    done

    if [ "$#" -gt 0 ]; then
        if [ -n "$remove_email_arg" ]; then
            printf 'Unexpected argument: %s\n' "$1" >&2
            server_user_remove_usage 1
        fi
        remove_email_arg="$1"
        shift
    fi

    if [ "$#" -gt 0 ]; then
        printf 'Unexpected argument: %s\n' "$1" >&2
        server_user_remove_usage 1
    fi

    server_user_require_inbounds
    server_user_init_paths

    xray_check_repo_access 'scripts/server_user.sh remove'

    server_user_show_clients
    printf '\n'

    EMAIL="$remove_email_arg"
    if [ -z "$EMAIL" ] && [ -n "${XRAY_CLIENT_EMAIL:-}" ]; then
        EMAIL="$XRAY_CLIENT_EMAIL"
    fi

    if [ -z "$EMAIL" ]; then
        if [ -t 0 ] || [ -r /dev/tty ]; then
            EMAIL="$(server_user_prompt_value 'Enter client email to remove' '')"
        else
            xray_die "Client email not provided. Pass as an argument or set XRAY_CLIENT_EMAIL."
        fi
    fi

    EMAIL="$(printf '%s' "$EMAIL" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"

    [ -n "$EMAIL" ] || xray_die "Client email cannot be empty."
    printf '%s' "$EMAIL" | grep -Eq '^[^[:space:]]+$' || xray_die "Client email must not contain whitespace."

    CLIENTS_PRESENT=0
    CLIENTS_MATCHES=0
    CLIENT_ID=""
    CLIENT_STATUS=""
    if [ -f "$SERVER_USER_CLIENTS_FILE" ]; then
        CLIENTS_PRESENT=1
        if ! jq empty "$SERVER_USER_CLIENTS_FILE" >/dev/null 2>&1; then
            xray_die "Existing $SERVER_USER_CLIENTS_FILE contains invalid JSON."
        fi
        CLIENTS_MATCHES=$(jq -r --arg email "$EMAIL" '
            ([ .[]? | select((.email // "") == $email) ] | length)
        ' "$SERVER_USER_CLIENTS_FILE")
        if [ "$CLIENTS_MATCHES" -gt 0 ]; then
            CLIENT_ID=$(jq -r --arg email "$EMAIL" '
                first(.[]? | select((.email // "") == $email) | .id) // ""
            ' "$SERVER_USER_CLIENTS_FILE")
            CLIENT_STATUS=$(jq -r --arg email "$EMAIL" '
                first(.[]? | select((.email // "") == $email) | .status) // ""
            ' "$SERVER_USER_CLIENTS_FILE")
        fi
    else
        xray_log "Clients registry file $SERVER_USER_CLIENTS_FILE not found; will update inbound configuration only."
    fi

    INBOUND_MATCHES=$(jq -r --arg email "$EMAIL" '
        [ (.inbounds // [])[]
          | (.settings.clients // [])[]
          | select((.email // "") == $email)
        ]
        | length
    ' "$SERVER_USER_INBOUNDS_FILE")

    if [ "$CLIENTS_MATCHES" -eq 0 ] && [ "$INBOUND_MATCHES" -eq 0 ]; then
        xray_die "Client '$EMAIL' not found in $SERVER_USER_CLIENTS_FILE or $SERVER_USER_INBOUNDS_FILE"
    fi

    if [ "$CLIENTS_PRESENT" -eq 1 ] && [ "$CLIENTS_MATCHES" -gt 0 ]; then
        TMP_CLIENTS="$(mktemp)"
        jq --arg email "$EMAIL" '
            (. // []) | map(select((.email // "") != $email))
        ' "$SERVER_USER_CLIENTS_FILE" > "$TMP_CLIENTS"
        mv "$TMP_CLIENTS" "$SERVER_USER_CLIENTS_FILE"
        chmod 600 "$SERVER_USER_CLIENTS_FILE"
    fi

    if [ "$INBOUND_MATCHES" -gt 0 ]; then
        TMP_INBOUNDS="$(mktemp)"
        jq --arg email "$EMAIL" '
            .inbounds |= ( (. // []) | map(
                if (.protocol // "") == "trojan" then
                    .settings = (.settings // {})
                    | .settings.clients = ((.settings.clients // []) | map(select((.email // "") != $email)))
                else
                    .
                end
            ))
        ' "$SERVER_USER_INBOUNDS_FILE" > "$TMP_INBOUNDS"
        mv "$TMP_INBOUNDS" "$SERVER_USER_INBOUNDS_FILE"
        chmod 644 "$SERVER_USER_INBOUNDS_FILE"
    fi

    xray_restart_service "xray-p2p" "/etc/init.d/xray-p2p"

    if [ -n "$CLIENT_ID" ]; then
        if [ -n "$CLIENT_STATUS" ]; then
            xray_log "Client '$EMAIL' (id $CLIENT_ID, status $CLIENT_STATUS) removed."
        else
            xray_log "Client '$EMAIL' (id $CLIENT_ID) removed."
        fi
    else
        xray_log "Client '$EMAIL' removed from inbound configuration."
    fi
}