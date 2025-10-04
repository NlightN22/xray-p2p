#!/bin/sh
set -eu

SCRIPT_NAME=${0##*/}

usage() {
    cat <<EOF
Usage: $SCRIPT_NAME [options] [EMAIL]

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

email_arg=""

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
            if [ -n "$email_arg" ]; then
                printf 'Unexpected argument: %s\n' "$1" >&2
                usage 1
            fi
            email_arg="$1"
            ;;
    esac
    shift
done

if [ "$#" -gt 0 ]; then
    if [ -n "$email_arg" ]; then
        printf 'Unexpected argument: %s\n' "$1" >&2
        usage 1
    fi
    email_arg="$1"
    shift
fi

if [ "$#" -gt 0 ]; then
    printf 'Unexpected argument: %s\n' "$1" >&2
    usage 1
fi

if [ -z "${XRAY_SELF_DIR:-}" ]; then
    case "$0" in
        */*)
            XRAY_SELF_DIR=$(CDPATH= cd -- "$(dirname "$0")" 2>/dev/null && pwd)
            export XRAY_SELF_DIR
            ;;
    esac
fi

umask 077

XRAY_COMMON_LIB_PATH_DEFAULT="lib/common.sh"
XRAY_COMMON_LIB_REMOTE_PATH_DEFAULT="scripts/lib/common.sh"

load_common_lib() {
    lib_local="${XRAY_COMMON_LIB_PATH:-$XRAY_COMMON_LIB_PATH_DEFAULT}"
    lib_remote="${XRAY_COMMON_LIB_REMOTE_PATH:-$XRAY_COMMON_LIB_REMOTE_PATH_DEFAULT}"

    if [ -n "${XRAY_SELF_DIR:-}" ]; then
        candidate="${XRAY_SELF_DIR%/}/$lib_local"
        if [ -r "$candidate" ]; then
            # shellcheck disable=SC1090
            . "$candidate"
            return 0
        fi
    fi

    if [ -r "$lib_local" ]; then
        # shellcheck disable=SC1090
        . "$lib_local"
        return 0
    fi

    base="${XRAY_REPO_BASE_URL:-https://raw.githubusercontent.com/NlightN22/xray-p2p/main}"
    base_trimmed="${base%/}"
    case "$lib_remote" in
        /*)
            lib_url="${base_trimmed}${lib_remote}"
            ;;
        *)
            lib_url="${base_trimmed}/${lib_remote}"
            ;;
    esac

    tmp="$(mktemp 2>/dev/null)" || return 1
    if command -v curl >/dev/null 2>&1 && curl -fsSL "$lib_url" -o "$tmp"; then
        # shellcheck disable=SC1090
        . "$tmp"
        rm -f "$tmp"
        return 0
    fi
    if command -v wget >/dev/null 2>&1 && wget -q -O "$tmp" "$lib_url"; then
        # shellcheck disable=SC1090
        . "$tmp"
        rm -f "$tmp"
        return 0
    fi
    rm -f "$tmp"
    return 1
}

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

show_existing_clients() {
    list_remote_default="${XRAY_LIST_SCRIPT_PATH:-scripts/user_list.sh}"
    list_remote="${XRAY_LIST_SCRIPT_REMOTE_PATH:-$list_remote_default}"
    list_local="${XRAY_LIST_SCRIPT_LOCAL_PATH:-$(basename "$list_remote")}"

    if clients_output=$(XRAY_CONFIG_DIR="$CONFIG_DIR" \
            XRAY_INBOUNDS_FILE="$INBOUNDS_FILE" \
            XRAY_CLIENTS_FILE="$CLIENTS_FILE" \
            XRAY_SKIP_REPO_CHECK=1 \
            xray_run_repo_script optional "$list_local" "$list_remote" 2>&1); then
        if [ -n "$clients_output" ]; then
            log "Current clients (email password status):"
            printf '%s\n' "$clients_output"
        fi
        printf '\n'
        return
    fi

    if [ -n "$clients_output" ]; then
        printf '%s\n' "$clients_output" >&2
    fi
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

xray_check_repo_access 'scripts/user_remove.sh'

show_existing_clients

EMAIL="$email_arg"
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
