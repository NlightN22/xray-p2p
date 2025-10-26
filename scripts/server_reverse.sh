#!/bin/sh
set -eu

SCRIPT_NAME=${0##*/}

usage() {
    cat <<EOF
Usage:
  $SCRIPT_NAME                 List recorded reverse tunnels.
  $SCRIPT_NAME list            Same as default list action.
  $SCRIPT_NAME add [--subnet CIDR]... [--server HOST] [--id SLUG]
  $SCRIPT_NAME remove [--id SLUG] [--subnet CIDR] [--server HOST]

Environment:
  XRAY_REVERSE_SUFFIX          Domain/tag suffix (default: .rev).
  XRAY_REVERSE_TUNNEL_ID       Override tunnel identifier slug when adding.
  XRAY_REVERSE_SERVER_ID       Default external server identifier.
  XRAY_REVERSE_SUBNETS         Default comma/space separated subnets.
  XRAY_REVERSE_SUBNET          Alias for XRAY_REVERSE_SUBNETS.
  XRAY_CONFIG_DIR              XRAY configuration directory (default: /etc/xray-p2p).
  XRAY_ROUTING_FILE            Routing file path (default: $XRAY_CONFIG_DIR/routing.json).
  XRAY_ROUTING_TEMPLATE        Local routing template (default: config_templates/server/routing.json).
  XRAY_ROUTING_TEMPLATE_REMOTE Remote routing template within repository.
  XRAY_TUNNELS_DIR             Directory for metadata (default: $XRAY_CONFIG_DIR/config).
  XRAY_TUNNELS_FILE            Metadata file path (default: $XRAY_TUNNELS_DIR/tunnels.json).
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

REVERSE_COMMON_LOCAL="${XRAY_REVERSE_COMMON_LIB:-lib/reverse_common.sh}"
REVERSE_COMMON_REMOTE="${XRAY_REVERSE_COMMON_REMOTE:-scripts/lib/reverse_common.sh}"

load_repo_lib "$REVERSE_COMMON_LOCAL" "$REVERSE_COMMON_REMOTE"
load_repo_lib "$SERVER_REVERSE_INPUTS_LOCAL" "$SERVER_REVERSE_INPUTS_REMOTE"
load_repo_lib "$SERVER_REVERSE_ROUTING_LOCAL" "$SERVER_REVERSE_ROUTING_REMOTE"
load_repo_lib "$SERVER_REVERSE_STORE_LOCAL" "$SERVER_REVERSE_STORE_REMOTE"

XRAY_CONFIG_DIR=${XRAY_CONFIG_DIR:-/etc/xray-p2p}
ROUTING_FILE=${XRAY_ROUTING_FILE:-$XRAY_CONFIG_DIR/routing.json}
ROUTING_TEMPLATE_LOCAL=${XRAY_ROUTING_TEMPLATE:-config_templates/server/routing.json}
ROUTING_TEMPLATE_REMOTE=${XRAY_ROUTING_TEMPLATE_REMOTE:-config_templates/server/routing.json}
TUNNELS_DIR=${XRAY_TUNNELS_DIR:-$XRAY_CONFIG_DIR/config}
TUNNELS_FILE=${XRAY_TUNNELS_FILE:-$TUNNELS_DIR/tunnels.json}

server_reverse_load_env_subnets() {
    server_reverse_subnet_reset
    if [ -n "${XRAY_REVERSE_SUBNET:-}" ]; then
        server_reverse_subnet_add_many "$XRAY_REVERSE_SUBNET"
    fi
    if [ -n "${XRAY_REVERSE_SUBNETS:-}" ]; then
        server_reverse_subnet_add_many "$XRAY_REVERSE_SUBNETS"
    fi
}

server_reverse_matches() {
    subnet_filter="$1"
    server_filter="$2"
    jq -r \
        --arg subnet "$subnet_filter" \
        --arg server "$server_filter" \
        '(. // [])
        | map(select(
            ($server == "" or (.server_id // "") == $server)
            and ($subnet == "" or ((.subnets // []) | index($subnet)) != null)
        ))
        | .[]
        | [(.tunnel_id // ""), (.server_id // ""), ((.subnets // []) | join(",")), (.domain // "")]
        | @tsv' "$TUNNELS_FILE"
}

server_reverse_choose_tunnel() {
    subnet_filter="$1"
    server_filter="$2"

    if [ ! -f "$TUNNELS_FILE" ]; then
        xray_die "Reverse tunnel metadata file not found: $TUNNELS_FILE"
    fi

    server_reverse_store_require "$TUNNELS_FILE"

    matches=$(server_reverse_matches "$subnet_filter" "$server_filter")
    if [ -z "$matches" ]; then
        xray_die "No reverse tunnels match the requested filters."
    fi

    tmp="$(mktemp 2>/dev/null)" || xray_die "Unable to create temporary file"
    printf '%s\n' "$matches" >"$tmp"
    count=$(wc -l <"$tmp" | tr -d ' \t\r')
    if [ "$count" -eq 0 ]; then
        rm -f "$tmp"
        xray_die "No reverse tunnels match the requested filters."
    fi

    if [ "$count" -eq 1 ]; then
        IFS='\t' read -r tunnel_id _rest <"$tmp"
        rm -f "$tmp"
        printf '%s' "$tunnel_id"
        return
    fi

    if [ ! -t 0 ] && [ ! -r /dev/tty ]; then
        rm -f "$tmp"
        xray_die "Multiple reverse tunnels match; specify --id, --server, or --subnet to disambiguate."
    fi

    printf 'Select reverse tunnel:\n' >&2
    i=1
    while IFS='\t' read -r tunnel_id match_server match_subnets match_domain; do
        [ -n "$match_server" ] || match_server="-"
        [ -n "$match_subnets" ] || match_subnets="-"
        [ -n "$match_domain" ] || match_domain="-"
        printf '  [%d] %s (server: %s, subnets: %s, domain: %s)\n' "$i" "$tunnel_id" "$match_server" "$match_subnets" "$match_domain" >&2
        i=$((i + 1))
    done <"$tmp"

    read_fd=0
    if [ -r /dev/tty ]; then
        exec 7</dev/tty
        read_fd=7
    fi

    while :; do
        printf 'Enter selection [1-%s]: ' "$count" >&2
        if [ "$read_fd" -eq 7 ]; then
            IFS= read -r choice <&7 || choice=""
        else
            IFS= read -r choice || choice=""
        fi
        case "$choice" in
            *[!0-9]*|"")
                printf 'Invalid selection.\n' >&2
                ;;
            *)
                if [ "$choice" -ge 1 ] && [ "$choice" -le "$count" ]; then
                    selected=$(sed -n "${choice}p" "$tmp")
                    IFS='\t' read -r tunnel_id _rest <<EOF
$selected
EOF
                    [ "$read_fd" -eq 7 ] && exec 7<&-
                    rm -f "$tmp"
                    printf '%s' "$tunnel_id"
                    return
                fi
                printf 'Selection out of range.\n' >&2
                ;;
        esac
    done
}

server_reverse_fetch_entry() {
    key="$1"
    jq -r --arg key "$key" '
        (. // [])
        | map(select((.tunnel_id // "") == $key))
        | .[0]
        | [(.tunnel_id // ""), (.server_id // ""), (.domain // ""), ((.subnets // []) | join(","))]
        | @tsv
    ' "$TUNNELS_FILE"
}

cmd_list() {
    if [ ! -f "$TUNNELS_FILE" ]; then
        printf 'No reverse tunnels recorded.\n'
        return 0
    fi
    server_reverse_store_require "$TUNNELS_FILE"
    server_reverse_store_print_table "$TUNNELS_FILE"
}

cmd_add() {
    tunnel_override=""
    server_arg=""

    server_reverse_load_env_subnets

    while [ "$#" -gt 0 ]; do
        case "$1" in
            -h|--help)
                usage 0
                ;;
            -s|--subnet)
                if [ "$#" -lt 2 ]; then
                    printf 'Option %s requires an argument.\n' "$1" >&2
                    usage 1
                fi
                value=$(server_reverse_trim_spaces "$2")
                if [ -z "$value" ]; then
                    xray_die 'Invalid subnet: (empty)'
                fi
                server_reverse_subnet_add "$value"
                shift
                ;;
            --subnet=*)
                value=$(server_reverse_trim_spaces "${1#*=}")
                if [ -z "$value" ]; then
                    xray_die 'Invalid subnet: (empty)'
                fi
                server_reverse_subnet_add "$value"
                ;;
            --server)
                if [ "$#" -lt 2 ]; then
                    printf 'Option %s requires an argument.\n' "$1" >&2
                    usage 1
                fi
                server_arg=$(server_reverse_trim_spaces "$2")
                shift
                ;;
            --server=*)
                server_arg=$(server_reverse_trim_spaces "${1#*=}")
                ;;
            --id)
                if [ "$#" -lt 2 ]; then
                    printf 'Option %s requires an argument.\n' "$1" >&2
                    usage 1
                fi
                tunnel_override=$(server_reverse_trim_spaces "$2")
                shift
                ;;
            --id=*)
                tunnel_override=$(server_reverse_trim_spaces "${1#*=}")
                ;;
            --)
                shift
                break
                ;;
            -* )
                printf 'Unknown option: %s\n' "$1" >&2
                usage 1
                ;;
            *)
                printf 'Unexpected argument: %s\n' "$1" >&2
                usage 1
                ;;
        esac
        shift
    done

    if [ "$#" -gt 0 ]; then
        printf 'Unexpected argument: %s\n' "$1" >&2
        usage 1
    fi

    server_id=$(server_reverse_read_server "$server_arg")
    server_reverse_validate_server "$server_id"

    server_reverse_prompt_subnets

    primary_subnet=$(server_reverse_subnet_primary || printf '')
    tunnel_id=$(reverse_resolve_tunnel_id "$primary_subnet" "$server_id" "$tunnel_override")

    suffix="${XRAY_REVERSE_SUFFIX:-.rev}"
    domain="$tunnel_id$suffix"
    tag="$domain"
    subnet_json=$(server_reverse_subnet_json)

    server_reverse_ensure_routing_file "$ROUTING_FILE" "$ROUTING_TEMPLATE_LOCAL" "$ROUTING_TEMPLATE_REMOTE"

    if [ -f "$TUNNELS_FILE" ] && server_reverse_store_has "$TUNNELS_FILE" "$tunnel_id"; then
        xray_log "Reverse tunnel '$tunnel_id' already exists; skipping."
        return 0
    fi

    server_reverse_update_routing "$ROUTING_FILE" "$tunnel_id" "$suffix" "$subnet_json" "$server_id"
    server_reverse_store_add "$TUNNELS_FILE" "$TUNNELS_DIR" "$tunnel_id" "$domain" "$tag" "$subnet_json" "$server_id"

    xray_restart_service "xray-p2p" "/etc/init.d/xray-p2p" ""
    xray_log "Reverse tunnel '$tunnel_id' recorded (server $server_id, domain $domain)."
}

cmd_remove() {
    id_arg=""
    subnet_arg=""
    server_arg=""

    while [ "$#" -gt 0 ]; do
        case "$1" in
            -h|--help)
                usage 0
                ;;
            --id)
                if [ "$#" -lt 2 ]; then
                    printf 'Option %s requires an argument.\n' "$1" >&2
                    usage 1
                fi
                id_arg=$(server_reverse_trim_spaces "$2")
                shift
                ;;
            --id=*)
                id_arg=$(server_reverse_trim_spaces "${1#*=}")
                ;;
            -s|--subnet)
                if [ "$#" -lt 2 ]; then
                    printf 'Option %s requires an argument.\n' "$1" >&2
                    usage 1
                fi
                subnet_arg=$(server_reverse_trim_spaces "$2")
                shift
                ;;
            --subnet=*)
                subnet_arg=$(server_reverse_trim_spaces "${1#*=}")
                ;;
            --server)
                if [ "$#" -lt 2 ]; then
                    printf 'Option %s requires an argument.\n' "$1" >&2
                    usage 1
                fi
                server_arg=$(server_reverse_trim_spaces "$2")
                shift
                ;;
            --server=*)
                server_arg=$(server_reverse_trim_spaces "${1#*=}")
                ;;
            --)
                shift
                break
                ;;
            -* )
                printf 'Unknown option: %s\n' "$1" >&2
                usage 1
                ;;
            *)
                printf 'Unexpected argument: %s\n' "$1" >&2
                usage 1
                ;;
        esac
        shift
    done

    if [ "$#" -gt 0 ]; then
        printf 'Unexpected argument: %s\n' "$1" >&2
        usage 1
    fi

    if [ -n "$subnet_arg" ] && ! validate_subnet "$subnet_arg"; then
        xray_die "Invalid subnet: $subnet_arg"
    fi

    if [ -n "$server_arg" ]; then
        server_reverse_validate_server "$server_arg"
    fi

    server_reverse_store_require "$TUNNELS_FILE"

    if [ -n "$id_arg" ]; then
        tunnel_id=$(reverse_resolve_tunnel_id "" "" "$id_arg")
        if ! server_reverse_store_has "$TUNNELS_FILE" "$tunnel_id"; then
            xray_die "Tunnel '$tunnel_id' not found in $TUNNELS_FILE"
        fi
    else
        tunnel_id=$(server_reverse_choose_tunnel "$subnet_arg" "$server_arg")
    fi

    entry=$(server_reverse_fetch_entry "$tunnel_id")
    if [ -z "$entry" ]; then
        xray_die "Tunnel '$tunnel_id' not found in $TUNNELS_FILE"
    fi

    IFS='\t' read -r entry_id entry_server entry_domain entry_subnets <<EOF
$entry
EOF

    suffix="${XRAY_REVERSE_SUFFIX:-.rev}"
    server_reverse_store_remove "$TUNNELS_FILE" "$tunnel_id"
    server_reverse_remove_routing "$ROUTING_FILE" "$tunnel_id" "$suffix"

    xray_restart_service "xray-p2p" "/etc/init.d/xray-p2p" ""
    xray_log "Reverse tunnel '$entry_id' removed (server ${entry_server:-"-"}, domain ${entry_domain:-"-"})."
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
