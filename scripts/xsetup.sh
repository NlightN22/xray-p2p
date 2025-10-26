#!/bin/sh
set -eu

SCRIPT_NAME=${0##*/}

usage() {
    cat <<USAGE
Usage: $SCRIPT_NAME [options] [SERVER_ADDR] [USER_NAME] [SERVER_LAN] [CLIENT_LAN]

Automate server and client setup for xray-p2p from the client router.

Options:
  -h, --help            Show this help message and exit.
  -u, --ssh-user USER   SSH user for the remote server (default: root).
  -p, --ssh-port PORT   SSH port for the remote server (default: 22).
  -s, --server-port PORT
                        External port exposed on the server (default: 8443).
  -C, --cert FILE       Path to existing certificate file for the server.
  -K, --key  FILE       Path to existing private key file for the server.

Environment variables:
  XRAY_REPO_BASE_URL       Override repository base (default GitHub).
  XRAY_SERVER_SSH_USER     Default SSH user.
  XRAY_SERVER_SSH_PORT     Default SSH port.
  XRAY_SERVER_PORT         External port to expose on the server (fallback 8443).
USAGE
    exit "${1:-0}"
}

trim() {
    printf '%s' "$1" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//'
}

prompt_required() {
    label="$1"
    default_value="$2"
    response=""
    while [ -z "$response" ]; do
        if [ -t 0 ]; then
            if [ -n "$default_value" ]; then
                printf '%s [%s]: ' "$label" "$default_value" >&2
            else
                printf '%s: ' "$label" >&2
            fi
            IFS= read -r response
        elif [ -r /dev/tty ]; then
            if [ -n "$default_value" ]; then
                printf '%s [%s]: ' "$label" "$default_value" >&2
            else
                printf '%s: ' "$label" >&2
            fi
            IFS= read -r response </dev/tty
        else
            if [ -n "$default_value" ]; then
                response="$default_value"
                break
            fi
            printf 'Error: %s is required and no interactive terminal is available.\n' "$label" >&2
            exit 1
        fi
        if [ -z "$response" ] && [ -n "$default_value" ]; then
            response="$default_value"
        fi
    done
    printf '%s' "$response"
}

require_cmd() {
    if ! command -v "$1" >/dev/null 2>&1; then
        printf 'Error: required command not found: %s\n' "$1" >&2
        exit 1
    fi
}

sanitize_subnet_for_entry() {
    printf '%s' "$1" | tr 'A-Z' 'a-z' | sed 's/[^0-9a-z]/_/g'
}

redirect_entry_path_for_subnet() {
    local subnet_clean
    subnet_clean=$(sanitize_subnet_for_entry "$1")
    printf '/etc/nftables.d/xray-transparent.d/xray_redirect_%s.entry' "$subnet_clean"
}

CLIENT_CONN_TMP_FILES=""

load_client_connection_lib() {
    if command -v client_connection_tag_from_url >/dev/null 2>&1; then
        return 0
    fi

    for candidate in \
        "scripts/lib/client_connection.sh" \
        "./scripts/lib/client_connection.sh" \
        "lib/client_connection.sh"; do
        if [ -r "$candidate" ]; then
            # shellcheck disable=SC1090
            . "$candidate"
            return 0
        fi
    done

    tmp="$(mktemp)" || {
        printf 'Error: unable to create temporary file for client connection library.\n' >&2
        return 1
    }

    if curl -fsSL "${BASE_URL:-https://raw.githubusercontent.com/NlightN22/xray-p2p/main}/scripts/lib/client_connection.sh" -o "$tmp"; then
        # shellcheck disable=SC1090
        . "$tmp"
        CLIENT_CONN_TMP_FILES="${CLIENT_CONN_TMP_FILES} $tmp"
        return 0
    fi

    printf 'Error: unable to download client connection library.\n' >&2
    rm -f "$tmp"
    return 1
}

SSH_USER=${XRAY_SERVER_SSH_USER:-root}
SSH_PORT=${XRAY_SERVER_SSH_PORT:-22}
SERVER_PORT=${XRAY_SERVER_PORT:-8443}
CERT_FILE=""
KEY_FILE=""

while [ "$#" -gt 0 ]; do
    case "$1" in
        -h|--help)
            usage 0
            ;;
        -u|--ssh-user)
            if [ "$#" -lt 2 ]; then
                printf 'Error: %s requires an argument.\n' "$1" >&2
                usage 1
            fi
            SSH_USER="$2"
            shift 2
            ;;
        -p|--ssh-port)
            if [ "$#" -lt 2 ]; then
                printf 'Error: %s requires an argument.\n' "$1" >&2
                usage 1
            fi
            SSH_PORT="$2"
            shift 2
            ;;
        -s|--server-port)
            if [ "$#" -lt 2 ]; then
                printf 'Error: %s requires an argument.\n' "$1" >&2
                usage 1
            fi
            SERVER_PORT="$2"
            shift 2
            ;;
        -C|--cert)
            if [ "$#" -lt 2 ]; then
                printf 'Error: %s requires an argument.\n' "$1" >&2
                usage 1
            fi
            CERT_FILE="$2"
            shift 2
            ;;
        -K|--key)
            if [ "$#" -lt 2 ]; then
                printf 'Error: %s requires an argument.\n' "$1" >&2
                usage 1
            fi
            KEY_FILE="$2"
            shift 2
            ;;
        --)
            shift
            break
            ;;
        -*)
            printf 'Error: unknown option %s\n' "$1" >&2
            usage 1
            ;;
        *)
            break
            ;;
    esac
done

SERVER_ADDR="${1:-}"
if [ "$#" -gt 0 ]; then shift; fi
USER_NAME="${1:-}"
if [ "$#" -gt 0 ]; then shift; fi
SERVER_LAN="${1:-}"
if [ "$#" -gt 0 ]; then shift; fi
CLIENT_LAN="${1:-}"
if [ "$#" -gt 0 ]; then shift; fi

if [ "$#" -gt 0 ]; then
    printf 'Error: unexpected arguments: %s\n' "$*" >&2
    usage 1
fi

SERVER_ADDR="$(trim "$SERVER_ADDR")"
USER_NAME="$(trim "$USER_NAME")"
SERVER_LAN="$(trim "$SERVER_LAN")"
CLIENT_LAN="$(trim "$CLIENT_LAN")"

case "$SSH_PORT" in
    ''|*[!0-9]*)
        printf 'Error: SSH port must be numeric. Got: %s\n' "$SSH_PORT" >&2
        exit 1
        ;;
    *)
        :
        ;;
esac

case "$SERVER_PORT" in
    ''|*[!0-9]*)
        printf 'Error: Server port must be numeric. Got: %s\n' "$SERVER_PORT" >&2
        exit 1
        ;;
    *)
        if [ "$SERVER_PORT" -le 0 ] || [ "$SERVER_PORT" -gt 65535 ]; then
            printf 'Error: Server port must be between 1 and 65535.\n' >&2
            exit 1
        fi
        ;;
esac

if [ -z "$SERVER_ADDR" ]; then
    SERVER_ADDR="$(prompt_required 'Enter server address (IP or hostname)' '')"
    # Ask also for server port if address was not preset
    SERVER_PORT_INPUT="$(prompt_required 'Enter server external port' "$SERVER_PORT")"
    case "$SERVER_PORT_INPUT" in
        ''|*[!0-9]*)
            printf 'Error: Server port must be numeric. Got: %s\n' "$SERVER_PORT_INPUT" >&2
            exit 1
            ;;
        *)
            if [ "$SERVER_PORT_INPUT" -le 0 ] || [ "$SERVER_PORT_INPUT" -gt 65535 ]; then
                printf 'Error: Server port must be between 1 and 65535.\n' >&2
                exit 1
            fi
            SERVER_PORT="$SERVER_PORT_INPUT"
            ;;
    esac
fi
if [ -z "$USER_NAME" ]; then
    USER_NAME="$(prompt_required 'Enter XRAY client username to create on server' '')"
fi
if [ -z "$SERVER_LAN" ]; then
    SERVER_LAN="$(prompt_required 'Enter server LAN subnet (e.g. 10.0.0.0/24)' '')"
fi
if [ -z "$CLIENT_LAN" ]; then
    CLIENT_LAN="$(prompt_required 'Enter client LAN subnet (e.g. 10.0.1.0/24)' '')"
fi

BASE_URL_DEFAULT="https://raw.githubusercontent.com/NlightN22/xray-p2p/main"
BASE_URL="${XRAY_REPO_BASE_URL:-$BASE_URL_DEFAULT}"
BASE_URL="${BASE_URL%/}"

require_cmd ssh
require_cmd curl
require_cmd opkg
require_cmd sed
require_cmd grep
require_cmd mktemp
require_cmd tee

REMOTE_LOG="$(mktemp)"
STATUS_FILE="$(mktemp)"
cleanup() {
    [ -f "$REMOTE_LOG" ] && rm -f "$REMOTE_LOG"
    [ -f "$STATUS_FILE" ] && rm -f "$STATUS_FILE"
    for tmp in $CLIENT_CONN_TMP_FILES; do
        [ -n "$tmp" ] && [ -f "$tmp" ] && rm -f "$tmp"
    done
}
trap cleanup EXIT INT TERM

printf '[local] Using repository base: %s\n' "$BASE_URL" >&2
printf '[local] Server: %s@%s:%s\n' "$SSH_USER" "$SERVER_ADDR" "$SSH_PORT" >&2
printf '[local] User to issue: %s\n' "$USER_NAME" >&2
printf '[local] Server LAN: %s\n' "$SERVER_LAN" >&2
printf '[local] Client LAN: %s\n' "$CLIENT_LAN" >&2
printf '[local] Server external port: %s\n' "$SERVER_PORT" >&2
if [ -n "$CERT_FILE" ] || [ -n "$KEY_FILE" ]; then
    if [ -n "$CERT_FILE" ] && [ -n "$KEY_FILE" ]; then
        printf '[local] Using provided certificate paths.\n' >&2
    else
        printf '[local] Warning: both --cert and --key must be provided; ignoring provided path(s).\n' >&2
        CERT_FILE=""
        KEY_FILE=""
    fi
fi

(
    set +e
    ssh -p "$SSH_PORT" "$SSH_USER@$SERVER_ADDR" \
        BASE_URL="$BASE_URL" \
        SERVER_ADDR="$SERVER_ADDR" \
        USER_NAME="$USER_NAME" \
        CLIENT_LAN="$CLIENT_LAN" \
        SERVER_PORT="$SERVER_PORT" \
        sh <<'EOS'
set -eu

log() {
    printf '[server] %s\n' "$1" >&2
}

require_cmd() {
    if ! command -v "$1" >/dev/null 2>&1; then
        log "Required command not found: $1"
        exit 1
    fi
}

require_cmd curl
require_cmd awk

if command -v opkg >/dev/null 2>&1; then
    have_opkg=1
else
    have_opkg=0
fi

SERVER_INSTALLED=0
if [ -x /etc/init.d/xray-p2p ] && [ -f /etc/xray-p2p/inbounds.json ]; then
    SERVER_INSTALLED=1
fi

if [ "$SERVER_INSTALLED" -eq 1 ]; then
    log "Detected existing xray-p2p deployment."

    if [ "$have_opkg" -eq 1 ]; then
        missing_pkgs=""
        if ! command -v jq >/dev/null 2>&1; then
            missing_pkgs="${missing_pkgs:+$missing_pkgs }jq"
        fi
        if ! command -v openssl >/dev/null 2>&1; then
            missing_pkgs="${missing_pkgs:+$missing_pkgs }openssl-util"
        fi
        if ! command -v nft >/dev/null 2>&1; then
            missing_pkgs="${missing_pkgs:+$missing_pkgs }nftables"
        fi
        if [ -n "$missing_pkgs" ]; then
            log "Installing missing dependencies: $missing_pkgs"
            opkg update
            # shellcheck disable=SC2086
            opkg install $missing_pkgs
        fi
    else
        log "Warning: opkg not available; cannot auto-install dependencies."
    fi

    if [ -x /etc/init.d/xray-p2p ]; then
        if ! /etc/init.d/xray-p2p status >/dev/null 2>&1; then
            log "Warning: xray-p2p service status check failed."
        else
            log "xray-p2p service appears to be running."
        fi
    fi
else
    if [ "$have_opkg" -ne 1 ]; then
        log "Error: opkg package manager is required to provision the server."
        exit 1
    fi

    log "xray-p2p not detected; provisioning server."
    opkg update
    opkg install jq openssl-util
    env XRAY_PORT="$SERVER_PORT" \
        curl -fsSL "$BASE_URL/scripts/server.sh" | sh -s -- install "$SERVER_ADDR" "$SERVER_PORT" \
        ${CERT_FILE:+--cert "$CERT_FILE"} ${KEY_FILE:+--key "$KEY_FILE"}
fi

if ! command -v jq >/dev/null 2>&1; then
    log "Error: jq command not available on server."
    exit 1
fi
if ! command -v openssl >/dev/null 2>&1; then
    log "Error: openssl command not available on server."
    exit 1
fi

CLIENTS_FILE="/etc/xray-p2p/config/clients.json"
existing_link=""
if [ -f "$CLIENTS_FILE" ]; then
    if jq -e . "$CLIENTS_FILE" >/dev/null 2>&1; then
        existing_link=$(jq -r --arg email "$USER_NAME" '
            def find_link($arr):
                ($arr // [])
                | map(select((.email // "") == $email) | .link)
                | map(select(type == "string" and . != ""))
                | if length == 0 then empty else .[0] end;

            if type == "array" then
                find_link(.)
            elif type == "object" and has("clients") then
                find_link(.clients)
            else
                empty
            end
        ' "$CLIENTS_FILE" 2>/dev/null || true)
        existing_link=$(printf '%s' "$existing_link" | tr -d '\n')
        if [ "$existing_link" = "null" ]; then
            existing_link=""
        fi
    else
        log "Warning: clients registry contains invalid JSON; ignoring existing entries."
    fi
fi

trojan_link=""
if [ -n "$existing_link" ]; then
    log "Client $USER_NAME already exists; reusing issued credentials."
    trojan_link="$existing_link"
else
    log "Issuing user $USER_NAME..."
    issue_output=$(curl -fsSL "$BASE_URL/scripts/server_user.sh" | sh -s -- issue "$USER_NAME" "$SERVER_ADDR")
    printf '%s\n' "$issue_output"

    trojan_link=$(printf '%s\n' "$issue_output" | awk '/^trojan:\/\// {link=$0} END {print link}')
    if [ -z "$trojan_link" ]; then
        log "Unable to detect trojan link in user_issue output"
        exit 1
    fi
fi
printf '__TROJAN_LINK__=%s\n' "$trojan_link"

log "Adding server reverse proxy..."
curl -fsSL "$BASE_URL/scripts/server_reverse.sh" | \
    env XRAY_REVERSE_SUBNET="$CLIENT_LAN" sh -s -- add "$USER_NAME"

sanitize_subnet_for_entry() {
    printf '%s' "$1" | tr 'A-Z' 'a-z' | sed 's/[^0-9a-z]/_/g'
}

redirect_entry_path_for_subnet() {
    local subnet_clean
    subnet_clean=$(sanitize_subnet_for_entry "$1")
    printf '/etc/nftables.d/xray-transparent.d/xray_redirect_%s.entry' "$subnet_clean"
}

redirect_entry_path=$(redirect_entry_path_for_subnet "$CLIENT_LAN")
if [ -f "$redirect_entry_path" ]; then
    existing_redirect_port=$(sed -n 's/^PORT="\(.*\)"/\1/p' "$redirect_entry_path" 2>/dev/null | head -n 1)
    if [ -n "$existing_redirect_port" ]; then
        log "Redirect for $CLIENT_LAN already present (port $existing_redirect_port); skipping."
    else
        log "Redirect for $CLIENT_LAN already present; skipping."
    fi
else
    log "Adding redirect for client LAN $CLIENT_LAN..."
    curl -fsSL "$BASE_URL/scripts/redirect.sh" | sh -s -- add "$CLIENT_LAN"
fi
EOS
    status=$?
    set -e
    printf '%s\n' "$status" >"$STATUS_FILE"
    exit "$status"
) | tee "$REMOTE_LOG"

ssh_status=$(cat "$STATUS_FILE")
if [ "$ssh_status" -ne 0 ]; then
    printf 'Error: remote server setup failed (exit %s).\n' "$ssh_status" >&2
    exit "$ssh_status"
fi

trojan_url=$(grep '__TROJAN_LINK__=' "$REMOTE_LOG" | tail -n 1 | sed 's/^.*__TROJAN_LINK__=//')
trojan_url="$(trim "$trojan_url")"
if [ -z "$trojan_url" ]; then
    printf 'Error: unable to extract trojan URL from remote output.\n' >&2
    exit 1
fi

printf '[local] Trojan URL captured: %s\n' "$trojan_url" >&2

if command -v jq >/dev/null 2>&1; then
    printf '[local] jq already present; skipping opkg update/install.\n' >&2
else
    if ! command -v opkg >/dev/null 2>&1; then
        printf '[local] Error: opkg package manager not found; cannot install jq.\n' >&2
        exit 1
    fi
    printf '[local] Updating package lists (opkg update)...\n' >&2
    opkg update

    printf '[local] Installing client dependencies (jq)...\n' >&2
    opkg install jq
fi

skip_port_check_env=""
if [ -d "/etc/xray-p2p" ] || [ -f "/etc/init.d/xray-p2p" ]; then
    skip_port_check_env="1"
    printf '[local] Existing xray-p2p assets detected; switching to incremental tunnel management.\n' >&2
fi
if [ -n "$skip_port_check_env" ]; then
    if ! load_client_connection_lib; then
        printf '[local] Error: unable to load client connection library.\n' >&2
        exit 1
    fi
    client_tag="$(client_connection_tag_from_url "$trojan_url")" || exit 1
    client_outbounds="/etc/xray-p2p/outbounds.json"
    if [ -f "$client_outbounds" ] && command -v jq >/dev/null 2>&1; then
        if jq -e --arg tag "$client_tag" '(.outbounds // []) | any(.[]?; (.tag // "") == $tag)' "$client_outbounds" >/dev/null 2>&1; then
            printf '[local] Existing outbound %s detected; refreshing via client_user remove/add.\n' "$client_tag" >&2
            curl -fsSL "$BASE_URL/scripts/client_user.sh" | sh -s -- remove "$client_tag" || true
        else
            printf '[local] Creating new outbound %s via client_user add.\n' "$client_tag" >&2
        fi
    else
        printf '[local] Outbound registry missing; continuing with client_user add.\n' >&2
    fi
    redirect_port=""
    redirect_entry=$(redirect_entry_path_for_subnet "$SERVER_LAN")
    if [ -f "$redirect_entry" ]; then
        redirect_port=$(sed -n 's/^PORT="\(.*\)"/\1/p' "$redirect_entry" 2>/dev/null | head -n 1 || true)
    fi
    if [ -z "$redirect_port" ] && [ -f "/etc/xray-p2p/inbounds.json" ] && command -v jq >/dev/null 2>&1; then
        redirect_port=$(jq -r '
            first(.inbounds[]? | select((.protocol // "") == "dokodemo-door") | .port) // empty
        ' /etc/xray-p2p/inbounds.json 2>/dev/null || true)
    fi
    if [ -n "$redirect_port" ]; then
        redirect_port=$(printf '%s' "$redirect_port" | tr -d ' \t\r\n')
    fi
    client_user_invoke() {
        set -- add "$trojan_url"
        if [ -n "$SERVER_LAN" ]; then
            set -- "$@" "$SERVER_LAN"
            if [ -n "$redirect_port" ]; then
                set -- "$@" "$redirect_port"
            fi
        fi
        curl -fsSL "$BASE_URL/scripts/client_user.sh" | sh -s -- "$@"
    }
    if ! client_user_invoke; then
        printf '[local] Error: unable to add outbound via client_user.sh; aborting to preserve existing configuration.\n' >&2
        exit 1
    fi
    if [ -x "/etc/init.d/xray-p2p" ]; then
        printf '[local] Restarting xray-p2p service...\n' >&2
        /etc/init.d/xray-p2p restart >/dev/null 2>&1 || /etc/init.d/xray-p2p restart
    fi
else
    printf '[local] Running client.sh install...\n' >&2
    curl -fsSL "$BASE_URL/scripts/client.sh" | sh -s -- install "$trojan_url"
fi

server_redirect_entry=$(redirect_entry_path_for_subnet "$SERVER_LAN")
if [ -f "$server_redirect_entry" ]; then
    existing_redirect_port=$(sed -n 's/^PORT="\(.*\)"/\1/p' "$server_redirect_entry" 2>/dev/null | head -n 1)
    if [ -n "$existing_redirect_port" ]; then
        printf '[local] Redirect for %s already present (port %s); skipping.\n' "$SERVER_LAN" "$existing_redirect_port" >&2
    else
        printf '[local] Redirect for %s already present; skipping.\n' "$SERVER_LAN" >&2
    fi
else
    printf '[local] Adding redirect for server LAN %s...\n' "$SERVER_LAN" >&2
    curl -fsSL "$BASE_URL/scripts/redirect.sh" | sh -s -- add "$SERVER_LAN"
fi

printf '[local] Adding client reverse proxy...\n' >&2
curl -fsSL "$BASE_URL/scripts/client_reverse.sh" | sh -s -- add "$USER_NAME"

printf '\nAll steps completed successfully.\n' >&2
