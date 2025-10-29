server_user_init_paths() {
    SERVER_USER_CONFIG_DIR="${XRAY_CONFIG_DIR:-/etc/xray-p2p}"
    SERVER_USER_INBOUNDS_FILE="${XRAY_INBOUNDS_FILE:-$SERVER_USER_CONFIG_DIR/inbounds.json}"
    SERVER_USER_CLIENTS_DIR="${XRAY_CLIENTS_DIR:-$SERVER_USER_CONFIG_DIR/config}"
    SERVER_USER_CLIENTS_FILE="${XRAY_CLIENTS_FILE:-$SERVER_USER_CLIENTS_DIR/clients.json}"
}

server_user_require_inbounds() {
    server_user_init_paths
    [ -f "$SERVER_USER_INBOUNDS_FILE" ] || xray_die "Inbound configuration not found: $SERVER_USER_INBOUNDS_FILE"
    xray_require_cmd jq
}

server_user_show_clients() {
    server_user_init_paths
    format="${XRAY_OUTPUT_MODE:-table}"
    if clients_output=$(XRAY_CONFIG_DIR="$SERVER_USER_CONFIG_DIR" \
            XRAY_INBOUNDS_FILE="$SERVER_USER_INBOUNDS_FILE" \
            XRAY_CLIENTS_FILE="$SERVER_USER_CLIENTS_FILE" \
            xray_run_repo_script optional "lib/user_list.sh" "scripts/lib/user_list.sh" 2>&1); then
        if [ -n "$clients_output" ]; then
            if [ "$format" != "json" ]; then
                xray_log "Current clients (email password status):"
            fi
            printf '%s\n' "$clients_output"
        fi
    elif [ -n "$clients_output" ]; then
        printf '%s\n' "$clients_output" >&2
    fi
}

server_user_prompt_value() {
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

server_user_generate_uuid() {
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

server_user_generate_password() {
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

server_user_generate_client_email() {
    prefix="${XRAY_EMAIL_PREFIX:-client}"
    domain="${XRAY_EMAIL_DOMAIN:-auto.local}"
    uuid_comp="$(server_user_generate_uuid | tr -d '-' | cut -c1-12)"
    if [ -z "$uuid_comp" ]; then
        uuid_comp="$(date -u '+%Y%m%d%H%M%S' 2>/dev/null || date '+%Y%m%d%H%M%S')"
    fi
    printf '%s-%s@%s' "$prefix" "$uuid_comp" "$domain"
}

server_user_format_link_host() {
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

server_user_run_network_interfaces_helper() {
    if [ "${XRAY_SHOW_INTERFACES:-1}" != "1" ]; then
        return
    fi

    helper_local="${XRAY_INTERFACES_SCRIPT_PATH:-lib/network_interfaces.sh}"
    helper_remote="${XRAY_INTERFACES_REMOTE_PATH:-scripts/lib/network_interfaces.sh}"

    if xray_run_repo_script optional "$helper_local" "$helper_remote" >&2; then
        printf '\n' >&2
    fi
}
