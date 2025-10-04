#!/bin/sh
set -eu

SCRIPT_NAME=${0##*/}

usage() {
    cat <<EOF_USAGE
Usage: $SCRIPT_NAME [options] [USERNAME]

Ensure the server reverse proxy routing is configured for USERNAME.

Options:
  -h, --help        Show this help message and exit.

Arguments:
  USERNAME          Optional XRAY client identifier; overrides env/prompt.

Environment variables:
  XRAY_REVERSE_USER         Preseed USERNAME when positional argument omitted.
  XRAY_REVERSE_SUFFIX       Override domain/tag suffix (default: .rev).
  XRAY_CONFIG_DIR           Path to XRAY configuration directory (default: /etc/xray).
  XRAY_ROUTING_FILE         Path to routing.json (defaults to ${XRAY_CONFIG_DIR:-/etc/xray}/routing.json).
  XRAY_ROUTING_TEMPLATE     Local template path for routing.json (default: config_templates/server/routing.json).
  XRAY_ROUTING_TEMPLATE_REMOTE  Remote template path relative to repo root (default: config_templates/server/routing.json).
EOF_USAGE
    exit "${1:-0}"
}

username_arg=""

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
            if [ -z "$username_arg" ]; then
                username_arg="$1"
            else
                printf 'Unexpected argument: %s\n' "$1" >&2
                usage 1
            fi
            ;;
    esac
    shift
done

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

# Ensure XRAY_SELF_DIR exists when invoked via stdin piping.
: "${XRAY_SELF_DIR:=}"

umask 077

COMMON_LIB_REMOTE_PATH="scripts/lib/common.sh"

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

CONFIG_DIR="${XRAY_CONFIG_DIR:-/etc/xray}"
ROUTING_FILE="${XRAY_ROUTING_FILE:-$CONFIG_DIR/routing.json}"
ROUTING_TEMPLATE_LOCAL="${XRAY_ROUTING_TEMPLATE:-config_templates/server/routing.json}"
ROUTING_TEMPLATE_REMOTE="${XRAY_ROUTING_TEMPLATE_REMOTE:-config_templates/server/routing.json}"

xray_require_cmd jq

ensure_routing_file() {
    if [ ! -f "$ROUTING_FILE" ]; then
        xray_seed_file_from_template "$ROUTING_FILE" "$ROUTING_TEMPLATE_REMOTE" "$ROUTING_TEMPLATE_LOCAL"
    fi
}

run_user_list() {
    xray_log "Existing XRAY users (user_list.sh):"
    if ! xray_run_repo_script optional "lib/user_list.sh" "scripts/lib/user_list.sh"; then
        xray_log "Unable to execute user_list.sh; continuing without user listing."
    fi
}

read_username() {
    value="$1"
    if [ -n "$value" ]; then
        printf '%s' "$value"
        return
    fi

    if [ -n "${XRAY_REVERSE_USER:-}" ]; then
        printf '%s' "$XRAY_REVERSE_USER"
        return
    fi

    read_fd=0
    if [ ! -t 0 ]; then
        if [ -r /dev/tty ]; then
            exec 3</dev/tty
            read_fd=3
        else
            xray_die "Username argument required; no interactive terminal available"
        fi
    fi

    while :; do
        printf 'Enter XRAY username: ' >&2
        if [ "$read_fd" -eq 3 ]; then
            IFS= read -r input <&3 || input=""
        else
            IFS= read -r input || input=""
        fi
        if [ -n "$input" ]; then
            printf '%s' "$input"
            return
        fi
        xray_log "Username cannot be empty."
    done
}

validate_username() {
    candidate="$1"
    case "$candidate" in
        ''|*[!A-Za-z0-9._-]*)
            xray_die "Username must contain only letters, digits, dot, underscore, or dash"
            ;;
    esac
}

update_routing() {
    username="$1"
    suffix="${XRAY_REVERSE_SUFFIX:-.rev}"
    domain="$username$suffix"
    tag="$domain"

    tmp="$(mktemp 2>/dev/null)" || xray_die "Unable to create temporary file"

    if ! jq \
        --arg user "$username" \
        --arg domain "$domain" \
        --arg tag "$tag" \
        '
        .reverse = (.reverse // {}) |
        .reverse.portals = (
            (.reverse.portals // [])
            | [ .[] | select(.domain != $domain) ]
            + [{ domain: $domain, tag: $tag }]
        ) |
        .routing = (.routing // {}) |
        (.routing.rules // []) as $rules |
        .routing.rules = (
            reduce $rules[] as $rule ([];
                if ($rule.outboundTag == $tag and ((($rule.domain // []) | index("full:" + $domain)) != null))
                then .
                else . + [$rule]
                end
            )
            + [{
                type: "field",
                domain: ["full:" + $domain],
                outboundTag: $tag,
                user: [$user]
            }]
        )
        ' "$ROUTING_FILE" >"$tmp"; then
        rm -f "$tmp"
        xray_die "Failed to update $ROUTING_FILE"
    fi

    chmod 0644 "$tmp"
    mv "$tmp" "$ROUTING_FILE"

    xray_log "Updated $ROUTING_FILE with reverse proxy entry for $username (tag: $tag)"
}

ensure_routing_file
run_user_list

USERNAME=$(read_username "$username_arg")
validate_username "$USERNAME"
update_routing "$USERNAME"

xray_log "Reverse proxy server install complete."
