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
            die "No interactive terminal available. Provide the required value via arguments or environment."
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

check_repo_access() {
    [ "${XRAY_SKIP_REPO_CHECK:-0}" = "1" ] && return

    base_url="${XRAY_REPO_BASE_URL:-https://raw.githubusercontent.com/NlightN22/xray-p2p/main}"
    check_path="${XRAY_REPO_CHECK_PATH:-scripts/remove_client.sh}"
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

EMAIL="${1:-}"
if [ -z "$EMAIL" ] && [ -n "${XRAY_CLIENT_EMAIL:-}" ]; then
    EMAIL="$XRAY_CLIENT_EMAIL"
fi

if [ -z "$EMAIL" ]; then
    if [ -t 0 ] || [ -r /dev/tty ]; then
        EMAIL="$(prompt_value 'Enter client email to remove' '')"
    else
        die "Client email not provided. Pass as an argument or set XRAY_CLIENT_EMAIL."
    fi
fi

EMAIL="$(printf '%s' "$EMAIL" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"

[ -n "$EMAIL" ] || die "Client email cannot be empty."
printf '%s' "$EMAIL" | grep -Eq '^[^[:space:]]+$' || die "Client email must not contain whitespace."

CLIENTS_MATCHES=0
CLIENT_ID=""
CLIENT_STATUS=""
CLIENTS_PRESENT=0
if [ -f "$CLIENTS_FILE" ]; then
    CLIENTS_PRESENT=1
    if ! jq empty "$CLIENTS_FILE" >/dev/null 2>&1; then
        die "Existing $CLIENTS_FILE contains invalid JSON."
    fi
    CLIENTS_MATCHES=$(jq -r --arg email "$EMAIL" '
        ([ .[]? | select((.email // "") == $email) ] | length)
    ' "$CLIENTS_FILE")
    if [ "$CLIENTS_MATCHES" -gt 0 ]; then
        CLIENT_ID=$(jq -r --arg email "$EMAIL" '
            first(.[]? | select((.email // "") == $email) | .id) // ""
        ' "$CLIENTS_FILE")
        CLIENT_STATUS=$(jq -r --arg email "$EMAIL" '
            first(.[]? | select((.email // "") == $email) | .status) // ""
        ' "$CLIENTS_FILE")
    fi
else
    log "Clients registry file $CLIENTS_FILE not found; will update inbound configuration only."
fi

INBOUND_MATCHES=$(jq -r --arg email "$EMAIL" '
    [ (.inbounds // [])[]
      | (.settings.clients // [])[]
      | select((.email // "") == $email)
    ]
    | length
' "$INBOUNDS_FILE")

if [ "$CLIENTS_MATCHES" -eq 0 ] && [ "$INBOUND_MATCHES" -eq 0 ]; then
    die "Client '$EMAIL' not found in $CLIENTS_FILE or $INBOUNDS_FILE"
fi

if [ "$CLIENTS_PRESENT" -eq 1 ] && [ "$CLIENTS_MATCHES" -gt 0 ]; then
    TMP_CLIENTS="$(mktemp)"
    jq --arg email "$EMAIL" '
        (. // []) | map(select((.email // "") != $email))
    ' "$CLIENTS_FILE" > "$TMP_CLIENTS"
    mv "$TMP_CLIENTS" "$CLIENTS_FILE"
    chmod 600 "$CLIENTS_FILE"
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
    ' "$INBOUNDS_FILE" > "$TMP_INBOUNDS"
    mv "$TMP_INBOUNDS" "$INBOUNDS_FILE"
    chmod 644 "$INBOUNDS_FILE"
fi

if [ "${XRAY_SKIP_RESTART:-0}" = "1" ]; then
    log "Skipping ${SERVICE_NAME} restart (XRAY_SKIP_RESTART=1)."
else
    SERVICE_SCRIPT="/etc/init.d/$SERVICE_NAME"
    [ -x "$SERVICE_SCRIPT" ] || die "Service script not found or not executable: $SERVICE_SCRIPT"
    log "Restarting $SERVICE_NAME service"
    "$SERVICE_SCRIPT" restart || die "Failed to restart $SERVICE_NAME service."
fi

if [ -n "$CLIENT_ID" ]; then
    log "Client '$EMAIL' (id $CLIENT_ID${CLIENT_STATUS:+, status $CLIENT_STATUS}) removed."
else
    log "Client '$EMAIL' removed from inbound configuration."
fi
