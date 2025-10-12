#!/bin/sh
set -eu

SCRIPT_NAME=${0##*/}

usage() {
    cat <<USAGE
Usage: $SCRIPT_NAME [options] [SERVER_ADDR] [USER_NAME] [SERVER_LAN] [CLIENT_LAN]

Automate server and client setup for xray-p2p from the client router.

Options:
  -h, --help          Show this help message and exit.
  -u, --ssh-user USER SSH user for the remote server (default: root).
  -p, --ssh-port PORT SSH port for the remote server (default: 22).

Environment variables:
  XRAY_REPO_BASE_URL       Override repository base (default GitHub).
  XRAY_SERVER_SSH_USER     Default SSH user.
  XRAY_SERVER_SSH_PORT     Default SSH port.
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

SSH_USER=${XRAY_SERVER_SSH_USER:-root}
SSH_PORT=${XRAY_SERVER_SSH_PORT:-22}

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

if [ -z "$SERVER_ADDR" ]; then
    SERVER_ADDR="$(prompt_required 'Enter server address (IP or hostname)' '')"
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
}
trap cleanup EXIT INT TERM

printf '[local] Using repository base: %s\n' "$BASE_URL" >&2
printf '[local] Server: %s@%s:%s\n' "$SSH_USER" "$SERVER_ADDR" "$SSH_PORT" >&2
printf '[local] User to issue: %s\n' "$USER_NAME" >&2
printf '[local] Server LAN: %s\n' "$SERVER_LAN" >&2
printf '[local] Client LAN: %s\n' "$CLIENT_LAN" >&2

(
    set +e
    ssh -p "$SSH_PORT" "$SSH_USER@$SERVER_ADDR" \
        BASE_URL="$BASE_URL" \
        SERVER_ADDR="$SERVER_ADDR" \
        USER_NAME="$USER_NAME" \
        CLIENT_LAN="$CLIENT_LAN" \
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
    curl -fsSL "$BASE_URL/scripts/server_install.sh" | sh -s -- "$SERVER_ADDR"
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
    issue_output=$(curl -fsSL "$BASE_URL/scripts/user_issue.sh" | sh -s -- "$USER_NAME" "$SERVER_ADDR")
    printf '%s\n' "$issue_output"

    trojan_link=$(printf '%s\n' "$issue_output" | awk '/^trojan:\/\// {link=$0} END {print link}')
    if [ -z "$trojan_link" ]; then
        log "Unable to detect trojan link in user_issue output"
        exit 1
    fi
fi
printf '__TROJAN_LINK__=%s\n' "$trojan_link"

log "Adding server reverse proxy..."
curl -fsSL "$BASE_URL/scripts/server_reverse_add.sh" | \
    env XRAY_REVERSE_SUBNET="$CLIENT_LAN" sh -s -- "$USER_NAME"

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
    curl -fsSL "$BASE_URL/scripts/redirect_add.sh" | sh -s -- "$CLIENT_LAN"
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

printf '[local] Updating package lists (opkg update)...\n' >&2
opkg update

printf '[local] Installing client dependencies (jq)...\n' >&2
opkg install jq

printf '[local] Running client_install.sh...\n' >&2
curl -fsSL "$BASE_URL/scripts/client_install.sh" | sh -s -- "$trojan_url"

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
    curl -fsSL "$BASE_URL/scripts/redirect_add.sh" | sh -s -- "$SERVER_LAN"
fi

printf '[local] Adding client reverse proxy...\n' >&2
curl -fsSL "$BASE_URL/scripts/client_reverse_add.sh" | sh -s -- "$USER_NAME"

printf '\nAll steps completed successfully.\n' >&2
