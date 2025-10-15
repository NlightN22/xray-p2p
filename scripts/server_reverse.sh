#!/bin/sh
######################
set -eu

SCRIPT_NAME=${0##*/}

usage() {
    cat <<EOF
Usage:
  $SCRIPT_NAME                 List recorded reverse tunnels.
  $SCRIPT_NAME list            Same as default list action.
  $SCRIPT_NAME add [options] [USERNAME]
  $SCRIPT_NAME remove USERNAME

Options for add:
  -h, --help        Show this help message and exit.
  -s, --subnet CIDR Add CIDR subnet to routing (repeatable).

Environment:
  XRAY_REVERSE_USER           Default username when omitted.
  XRAY_REVERSE_SUFFIX         Domain/tag suffix (default: .rev).
  XRAY_REVERSE_SUBNETS        Default comma/space separated subnets.
  XRAY_REVERSE_SUBNET         Alias for XRAY_REVERSE_SUBNETS.
  XRAY_CONFIG_DIR             XRAY configuration directory (default: /etc/xray-p2p).
  XRAY_ROUTING_FILE           Routing file path (default: \$XRAY_CONFIG_DIR/routing.json).
  XRAY_ROUTING_TEMPLATE       Local routing template to seed when missing.
  XRAY_ROUTING_TEMPLATE_REMOTE Remote template to seed when missing.
  XRAY_TUNNELS_DIR            Directory for metadata (default: \$XRAY_CONFIG_DIR/config).
  XRAY_TUNNELS_FILE           Metadata file path (default: \$XRAY_TUNNELS_DIR/tunnels.json).
EOF
    exit "${1:-0}"
}

if [ -z "${XRAY_SELF_DIR:-}" ]; then
    case "$0" in
        */*)
            XRAY_SELF_DIR=$(CDPATH= cd -- "$(dirname "$0")" 2>/dev/null && pwd)
            export XRAY_SELF_DIR
            ;;
    esac
fi

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
    tmp="$(mktemp 2>/dev/null)"
    if [ -z "$tmp" ]; then
        printf 'Error: Unable to create temporary loader script.\n' >&2
        exit 1
    fi
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

SERVER_REVERSE_LIB_TMP=""

cleanup_repo_libs() {
    for tmp in $SERVER_REVERSE_LIB_TMP; do
        if [ -n "$tmp" ] && [ -f "$tmp" ]; then
            rm -f "$tmp"
        fi
    done
}

trap cleanup_repo_libs EXIT
trap 'cleanup_repo_libs; exit 1' INT TERM HUP

load_repo_lib() {
    local local_spec="$1"
    local remote_spec="$2"
    local resolved=""
    local tmp=""

    if resolved=$(xray_resolve_local_path "$local_spec" 2>/dev/null) && [ -r "$resolved" ]; then
        # shellcheck disable=SC1090
        . "$resolved"
        return 0
    fi

    tmp="$(xray_fetch_repo_script "$remote_spec")" || xray_die "Required library not available: $remote_spec"
    SERVER_REVERSE_LIB_TMP="${SERVER_REVERSE_LIB_TMP} $tmp"
    # shellcheck disable=SC1090
    . "$tmp"
}

SERVER_REVERSE_INPUTS_LOCAL="${XRAY_SERVER_REVERSE_INPUTS_LIB:-lib/server_reverse_inputs.sh}"
SERVER_REVERSE_INPUTS_REMOTE="${XRAY_SERVER_REVERSE_INPUTS_REMOTE:-scripts/lib/server_reverse_inputs.sh}"
SERVER_REVERSE_ROUTING_LOCAL="${XRAY_SERVER_REVERSE_ROUTING_LIB:-lib/server_reverse_routing.sh}"
SERVER_REVERSE_ROUTING_REMOTE="${XRAY_SERVER_REVERSE_ROUTING_REMOTE:-scripts/lib/server_reverse_routing.sh}"
SERVER_REVERSE_STORE_LOCAL="${XRAY_SERVER_REVERSE_STORE_LIB:-lib/server_reverse_store.sh}"
SERVER_REVERSE_STORE_REMOTE="${XRAY_SERVER_REVERSE_STORE_REMOTE:-scripts/lib/server_reverse_store.sh}"

load_repo_lib "$SERVER_REVERSE_INPUTS_LOCAL" "$SERVER_REVERSE_INPUTS_REMOTE"
load_repo_lib "$SERVER_REVERSE_ROUTING_LOCAL" "$SERVER_REVERSE_ROUTING_REMOTE"
load_repo_lib "$SERVER_REVERSE_STORE_LOCAL" "$SERVER_REVERSE_STORE_REMOTE"

CONFIG_DIR="${XRAY_CONFIG_DIR:-/etc/xray-p2p}"
ROUTING_FILE="${XRAY_ROUTING_FILE:-$CONFIG_DIR/routing.json}"
ROUTING_TEMPLATE_REMOTE="${XRAY_ROUTING_TEMPLATE_REMOTE:-}"
ROUTING_TEMPLATE_LOCAL="${XRAY_ROUTING_TEMPLATE:-}"

TUNNELS_DIR="${XRAY_TUNNELS_DIR:-$CONFIG_DIR/config}"
TUNNELS_FILE="${XRAY_TUNNELS_FILE:-$TUNNELS_DIR/tunnels.json}"

xray_require_cmd jq

cmd_list() {
    if [ ! -f "$TUNNELS_FILE" ]; then
        printf 'No server reverse tunnels recorded.\n'
        return
    fi

    server_reverse_store_require "$TUNNELS_FILE"

    if [ "$(jq 'length' "$TUNNELS_FILE")" -eq 0 ]; then
        printf 'No server reverse tunnels recorded.\n'
        return
    fi

    server_reverse_store_print_table "$TUNNELS_FILE"
}

cmd_add() {
    SERVER_REVERSE_SUBNETS=""
    server_reverse_subnet_add_many "${XRAY_REVERSE_SUBNETS:-}"
    server_reverse_subnet_add_many "${XRAY_REVERSE_SUBNET:-}"

    username_arg=""

    while [ "$#" -gt 0 ]; do
        case "$1" in
            -h|--help)
                usage 0
                ;;
            -s|--subnet)
                if [ -n "${2:-}" ]; then
                    value=$(server_reverse_trim_spaces "$2")
                    if [ -z "$value" ]; then
                        xray_die "Invalid subnet: (empty)"
                    fi
                    server_reverse_subnet_add "$value"
                    shift
                else
                    printf 'Option %s requires an argument.\n' "$1" >&2
                    usage 1
                fi
                ;;
            --subnet=*)
                value=$(server_reverse_trim_spaces "${1#*=}")
                if [ -z "$value" ]; then
                    xray_die "Invalid subnet: (empty)"
                fi
                server_reverse_subnet_add "$value"
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

    server_reverse_ensure_routing_file "$ROUTING_FILE" "$ROUTING_TEMPLATE_LOCAL" "$ROUTING_TEMPLATE_REMOTE"
    server_reverse_run_user_list

    USERNAME=$(server_reverse_read_username "$username_arg")
    server_reverse_validate_username "$USERNAME"

    server_reverse_prompt_subnets

    suffix="${XRAY_REVERSE_SUFFIX:-.rev}"
    domain="$USERNAME$suffix"
    tag="$domain"
    subnet_json=$(server_reverse_subnet_json)

    server_reverse_update_routing "$ROUTING_FILE" "$USERNAME" "$suffix" "$subnet_json"
    server_reverse_store_add "$TUNNELS_FILE" "$TUNNELS_DIR" "$USERNAME" "$domain" "$tag" "$subnet_json"

    xray_restart_service "xray-p2p" "/etc/init.d/xray-p2p" ""
    xray_log "Reverse tunnel '$USERNAME' recorded with domain $domain."
}

cmd_remove() {
    if [ "$#" -ne 1 ]; then
        printf 'remove command expects exactly one USERNAME argument.\n' >&2
        usage 1
    fi

    USERNAME=$(server_reverse_trim_spaces "$1")
    if [ -z "$USERNAME" ]; then
        xray_die "Username cannot be empty."
    fi

    server_reverse_store_require "$TUNNELS_FILE"

    if ! server_reverse_store_has "$TUNNELS_FILE" "$USERNAME"; then
        xray_die "Tunnel '$USERNAME' not found in $TUNNELS_FILE"
    fi

    suffix="${XRAY_REVERSE_SUFFIX:-.rev}"
    domain="$USERNAME$suffix"
    server_reverse_store_remove "$TUNNELS_FILE" "$USERNAME"
    server_reverse_remove_routing "$ROUTING_FILE" "$USERNAME" "$suffix"

    xray_restart_service "xray-p2p" "/etc/init.d/xray-p2p" ""
    xray_log "Reverse tunnel '$USERNAME' (domain $domain) removed."
}

main() {
    if [ "$#" -eq 0 ]; then
        cmd_list
        return
    fi

    command="$1"
    shift

    case "$command" in
        -h|--help)
            usage 0
            ;;
        list)
            if [ "$#" -gt 0 ]; then
                printf 'list command does not take arguments.\n' >&2
                usage 1
            fi
            cmd_list
            ;;
        add)
            cmd_add "$@"
            ;;
        remove)
            cmd_remove "$@"
            ;;
        *)
            printf 'Unknown command: %s\n' "$command" >&2
            usage 1
            ;;
    esac
}

main "$@"
