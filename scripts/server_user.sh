#!/bin/sh
set -eu

SCRIPT_NAME=${0##*/}

usage() {
    cat <<EOF
Usage:
  $SCRIPT_NAME                 List registered XRAY clients.
  $SCRIPT_NAME list            Same as default list action.
  $SCRIPT_NAME issue [options] [EMAIL] [SERVER_ADDRESS]
  $SCRIPT_NAME remove [options] [EMAIL]

Run \`$SCRIPT_NAME <command> --help\` for command specific options.
EOF
    exit "${1:-0}"
}

if [ -z "${XRAY_SELF_DIR:-}" ]; then
    case "$0" in
        */*)
            XRAY_SELF_DIR=$(CDPATH= cd -- "$(dirname "$0")" 2>/dev/null && pwd)
            ;;
    esac
fi

: "${XRAY_SELF_DIR:=}"

umask 077

if ! command -v load_common_lib >/dev/null 2>&1; then
    for candidate in \
        "${XRAY_SELF_DIR%/}/scripts/lib/common_loader.sh" \
        "scripts/lib/common_loader.sh" \
        "lib/common_loader.sh"; do
        if [ -r "$candidate" ]; then
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

SERVER_USER_LIB_TMP=""

server_user_cleanup_libs() {
    for tmp in $SERVER_USER_LIB_TMP; do
        if [ -n "$tmp" ] && [ -f "$tmp" ]; then
            rm -f "$tmp"
        fi
    done
}

trap server_user_cleanup_libs EXIT INT TERM

server_user_load_lib() {
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
    SERVER_USER_LIB_TMP="${SERVER_USER_LIB_TMP} $tmp"
    # shellcheck disable=SC1090
    . "$tmp"
}

SERVER_USER_COMMON_LOCAL="${XRAY_SERVER_USER_COMMON_LIB:-lib/server_user_common.sh}"
SERVER_USER_COMMON_REMOTE="${XRAY_SERVER_USER_COMMON_REMOTE:-scripts/lib/server_user_common.sh}"
SERVER_USER_ISSUE_LOCAL="${XRAY_SERVER_USER_ISSUE_LIB:-lib/server_user_issue.sh}"
SERVER_USER_ISSUE_REMOTE="${XRAY_SERVER_USER_ISSUE_REMOTE:-scripts/lib/server_user_issue.sh}"
SERVER_USER_REMOVE_LOCAL="${XRAY_SERVER_USER_REMOVE_LIB:-lib/server_user_remove.sh}"
SERVER_USER_REMOVE_REMOTE="${XRAY_SERVER_USER_REMOVE_REMOTE:-scripts/lib/server_user_remove.sh}"

server_user_load_lib "$SERVER_USER_COMMON_LOCAL" "$SERVER_USER_COMMON_REMOTE"
server_user_load_lib "$SERVER_USER_ISSUE_LOCAL" "$SERVER_USER_ISSUE_REMOTE"
server_user_load_lib "$SERVER_USER_REMOVE_LOCAL" "$SERVER_USER_REMOVE_REMOTE"

server_user_cmd_list() {
    server_user_require_inbounds
    server_user_show_clients
}

main() {
    if [ "$#" -eq 0 ]; then
        server_user_cmd_list
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
            server_user_cmd_list
            ;;
        issue)
            server_user_cmd_issue "$@"
            ;;
        remove)
            server_user_cmd_remove "$@"
            ;;
        *)
            printf 'Unknown command: %s\n' "$command" >&2
            usage 1
            ;;
    esac
}

main "$@"
