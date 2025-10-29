#!/bin/sh
set -eu

SCRIPT_NAME=${0##*/}

usage() {
    cat <<EOF
Usage:
  $SCRIPT_NAME [--json]        List recorded client reverse tunnels.
  $SCRIPT_NAME list            Same as default list action.
  $SCRIPT_NAME add [--subnet CIDR] [--server HOST] [--id SLUG] [--outbound TAG]
  $SCRIPT_NAME remove [--id SLUG] [--server HOST]

Global options:
  --json                       Emit JSON instead of a table.

Environment:
  XRAY_REVERSE_SUFFIX          Domain/tag suffix (default: .rev).
  XRAY_REVERSE_TUNNEL_ID       Override tunnel identifier slug (subnet--server).
  XRAY_REVERSE_SERVER_ID       External server identifier used for slug defaults.
  XRAY_CONFIG_DIR             XRAY configuration directory (default: /etc/xray-p2p).
  XRAY_ROUTING_FILE           Routing file path (default: $XRAY_CONFIG_DIR/routing.json).
  XRAY_ROUTING_TEMPLATE       Local routing template (default: config_templates/client/routing.json).
  XRAY_ROUTING_TEMPLATE_REMOTE Remote template location relative to repo root.
  XRAY_CLIENT_REVERSE_DIR     Directory for metadata (default: $XRAY_CONFIG_DIR/config).
  XRAY_CLIENT_REVERSE_FILE    Metadata file path (default: $XRAY_CLIENT_REVERSE_DIR/client_reverse.json).
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

CLIENT_REVERSE_LIB_TMP=""

cleanup_repo_libs() {
    for tmp in $CLIENT_REVERSE_LIB_TMP; do
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
    CLIENT_REVERSE_LIB_TMP="${CLIENT_REVERSE_LIB_TMP} $tmp"
    # shellcheck disable=SC1090
    . "$tmp"
}

REVERSE_COMMON_LOCAL="${XRAY_REVERSE_COMMON_LIB:-lib/reverse_common.sh}"
REVERSE_COMMON_REMOTE="${XRAY_REVERSE_COMMON_REMOTE:-scripts/lib/reverse_common.sh}"
CLIENT_REVERSE_INPUTS_LOCAL="${XRAY_CLIENT_REVERSE_INPUTS_LIB:-lib/client_reverse_inputs.sh}"
CLIENT_REVERSE_INPUTS_REMOTE="${XRAY_CLIENT_REVERSE_INPUTS_REMOTE:-scripts/lib/client_reverse_inputs.sh}"
CLIENT_REVERSE_ROUTING_LOCAL="${XRAY_CLIENT_REVERSE_ROUTING_LIB:-lib/client_reverse_routing.sh}"
CLIENT_REVERSE_ROUTING_REMOTE="${XRAY_CLIENT_REVERSE_ROUTING_REMOTE:-scripts/lib/client_reverse_routing.sh}"
CLIENT_REVERSE_STORE_LOCAL="${XRAY_CLIENT_REVERSE_STORE_LIB:-lib/client_reverse_store.sh}"
CLIENT_REVERSE_STORE_REMOTE="${XRAY_CLIENT_REVERSE_STORE_REMOTE:-scripts/lib/client_reverse_store.sh}"

load_repo_lib "$REVERSE_COMMON_LOCAL" "$REVERSE_COMMON_REMOTE"
load_repo_lib "$CLIENT_REVERSE_INPUTS_LOCAL" "$CLIENT_REVERSE_INPUTS_REMOTE"
load_repo_lib "$CLIENT_REVERSE_ROUTING_LOCAL" "$CLIENT_REVERSE_ROUTING_REMOTE"
load_repo_lib "$CLIENT_REVERSE_STORE_LOCAL" "$CLIENT_REVERSE_STORE_REMOTE"

XRAY_CONFIG_DIR=${XRAY_CONFIG_DIR:-/etc/xray-p2p}
ROUTING_FILE=${XRAY_ROUTING_FILE:-$XRAY_CONFIG_DIR/routing.json}
ROUTING_TEMPLATE_LOCAL=${XRAY_ROUTING_TEMPLATE:-config_templates/client/routing.json}
ROUTING_TEMPLATE_REMOTE=${XRAY_ROUTING_TEMPLATE_REMOTE:-config_templates/client/routing.json}
CLIENT_REVERSE_DIR=${XRAY_CLIENT_REVERSE_DIR:-$XRAY_CONFIG_DIR/config}
CLIENT_REVERSE_FILE=${XRAY_CLIENT_REVERSE_FILE:-$CLIENT_REVERSE_DIR/client_reverse.json}
OUTBOUNDS_FILE=${XRAY_OUTBOUNDS_FILE:-$XRAY_CONFIG_DIR/outbounds.json}

client_reverse_is_integer() {
    case "$1" in
        ''|*[!0-9]*)
            return 1
            ;;
    esac
    return 0
}

client_reverse_split_server_identifier() {
    local identifier="$1"
    local host="$identifier"
    local port=""

    case "$identifier" in
        \[*\]:*)
            local candidate="${identifier##*:}"
            if client_reverse_is_integer "$candidate"; then
                port="$candidate"
                host="${identifier%:*}"
                host="${host#[}"
                host="${host%]}"
            fi
            ;;
        *:*:*)
            # Likely IPv6 without brackets; leave as-is to avoid mis-parsing.
            ;;
        *:*)
            local candidate="${identifier##*:}"
            if client_reverse_is_integer "$candidate"; then
                port="$candidate"
                host="${identifier%:*}"
            fi
            ;;
        *)
            ;;
    esac

    printf '%s\t%s\n' "$host" "$port"
}

client_reverse_sanitize_host() {
    printf '%s' "$1" \
        | tr '[:upper:]' '[:lower:]' \
        | sed 's/[^0-9a-z._-]/-/g; s/-\{2,\}/-/g; s/^-//; s/-$//'
}

client_reverse_try_jq_outbound_tag() {
    local server_host="$1"
    local expected_port="$2"
    local expected_tag="$3"
    local sanitized_host="$4"
    local candidate=""

    if ! command -v jq >/dev/null 2>&1; then
        return 1
    fi

    if [ -n "$expected_tag" ]; then
        candidate=$(jq -r \
            --arg tag "$expected_tag" '
            first(.outbounds[]? | select((.tag // "") == $tag) | (.tag // "")) // empty
            ' "$OUTBOUNDS_FILE" 2>/dev/null | tr -d '\r')
        if [ -n "$candidate" ] && [ "$candidate" != "null" ]; then
            printf '%s' "$candidate"
            return 0
        fi
    fi

    candidate=$(jq -r \
        --arg server "$server_host" \
        --arg port "$expected_port" '
        first(
            .outbounds[]? as $o
            | [($o.settings.servers // [])[]?
                | select(
                    ((.address // .server // "") == $server)
                    and ($port == "" or ((.port // "") | tostring) == $port)
                )
              ]
            | if length > 0 then ($o.tag // "") else empty end
        ) // empty
        ' "$OUTBOUNDS_FILE" 2>/dev/null | tr -d '\r')
    if [ -n "$candidate" ] && [ "$candidate" != "null" ]; then
        printf '%s' "$candidate"
        return 0
    fi

    candidate=$(jq -r \
        --arg server "$server_host" '
        first(
            .outbounds[]? as $o
            | [($o.settings.servers // [])[]?
                | select((.address // .server // "") == $server)
              ]
            | if length > 0 then ($o.tag // "") else empty end
        ) // empty
        ' "$OUTBOUNDS_FILE" 2>/dev/null | tr -d '\r')
    if [ -n "$candidate" ] && [ "$candidate" != "null" ]; then
        printf '%s' "$candidate"
        return 0
    fi

    if [ -n "$sanitized_host" ]; then
        candidate=$(jq -r \
            --arg fragment "$sanitized_host" '
            first(
                .outbounds[]?
                | (.tag // "")
                | select(length > 0)
                | select(index($fragment) != null)
            ) // empty
            ' "$OUTBOUNDS_FILE" 2>/dev/null | tr -d '\r')
        if [ -n "$candidate" ] && [ "$candidate" != "null" ]; then
            printf '%s' "$candidate"
            return 0
        fi
    fi

    candidate=$(jq -r '
        first(
            .outbounds[]?
            | (.tag // "")
            | select(length > 0)
        ) // empty
        ' "$OUTBOUNDS_FILE" 2>/dev/null | tr -d '\r')

    if [ -n "$candidate" ] && [ "$candidate" != "null" ]; then
        printf '%s' "$candidate"
        return 0
    fi

    return 1
}

resolve_client_outbound_tag() {
    local server_id="$1"
    local provided="$2"
    local candidate=""
    local attempt=0

    if [ -n "$provided" ]; then
        printf '%s' "$provided"
        return 0
    fi

    if [ -n "${XRAY_REVERSE_OUTBOUND_TAG:-}" ]; then
        printf '%s' "$XRAY_REVERSE_OUTBOUND_TAG"
        return 0
    fi

    while [ "$attempt" -lt 3 ]; do
        if [ -f "$OUTBOUNDS_FILE" ]; then
            local split host_part port_hint expected_port sanitized expected_tag
            host_part="$server_id"
            port_hint=""

            if split=$(client_reverse_split_server_identifier "$server_id"); then
                IFS='	' read -r host_part port_hint <<EOF
$split
EOF
            fi

            expected_port="$port_hint"
            if client_reverse_is_integer "${XRAY_REVERSE_SERVER_PORT:-}"; then
                expected_port="$XRAY_REVERSE_SERVER_PORT"
            elif client_reverse_is_integer "${XRAY_SERVER_PORT:-}"; then
                expected_port="$XRAY_SERVER_PORT"
            fi

            sanitized=$(client_reverse_sanitize_host "$host_part")
            expected_tag=""
            if [ -n "$sanitized" ] && [ -n "$expected_port" ]; then
                expected_tag="${sanitized}-${expected_port}"
            fi

            if client_reverse_try_jq_outbound_tag "$host_part" "$expected_port" "$expected_tag" "$sanitized"; then
                return 0
            fi
        fi

        attempt=$((attempt + 1))
        sleep 1
    done

    xray_die "Unable to determine outbound tag for server $server_id. Provide --outbound or set XRAY_REVERSE_OUTBOUND_TAG."
}

client_reverse_matches() {
    server_filter="$1"
    jq -r \
        --arg server "$server_filter" \
        '(. // [])
        | map(select($server == "" or (.server_id // "") == $server))
        | .[]
        | [(.tunnel_id // ""), (.server_id // ""), (.domain // ""), (.created_at // "" )]
        | @tsv' "$CLIENT_REVERSE_FILE"
}

client_reverse_choose_tunnel() {
    server_filter="$1"

    if [ ! -f "$CLIENT_REVERSE_FILE" ]; then
        xray_die "Client reverse metadata file not found: $CLIENT_REVERSE_FILE"
    fi

    client_reverse_store_require "$CLIENT_REVERSE_FILE"

    matches=$(client_reverse_matches "$server_filter")
    if [ -z "$matches" ]; then
        xray_die "No client reverse tunnels match the requested filters."
    fi

    tmp="$(mktemp 2>/dev/null)" || xray_die "Unable to create temporary file"
    printf '%s\n' "$matches" >"$tmp"
    count=$(wc -l <"$tmp" | tr -d ' \t\r')
    if [ "$count" -eq 0 ]; then
        rm -f "$tmp"
        xray_die "No client reverse tunnels match the requested filters."
    fi

    if [ "$count" -eq 1 ]; then
        IFS='\t' read -r tunnel_id _rest <"$tmp"
        rm -f "$tmp"
        printf '%s' "$tunnel_id"
        return
    fi

    if [ ! -t 0 ] && [ ! -r /dev/tty ]; then
        rm -f "$tmp"
        xray_die "Multiple client reverse tunnels match; specify --id or --server to disambiguate."
    fi

    printf 'Select client reverse tunnel:\n' >&2
    i=1
    while IFS='\t' read -r tunnel_id match_server match_domain match_created; do
        [ -n "$match_server" ] || match_server="-"
        [ -n "$match_domain" ] || match_domain="-"
        [ -n "$match_created" ] || match_created="-"
        printf '  [%d] %s (server: %s, domain: %s, created: %s)\n' "$i" "$tunnel_id" "$match_server" "$match_domain" "$match_created" >&2
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

client_reverse_fetch_entry() {
    key="$1"
    jq -r --arg key "$key" '
        (. // [])
        | map(select((.tunnel_id // "") == $key))
        | .[0]
        | [(.tunnel_id // ""), (.server_id // ""), (.outbound_tag // ""), (.domain // "")]
        | @tsv
    ' "$CLIENT_REVERSE_FILE"
}

cmd_list() {
    local format
    format="${XRAY_OUTPUT_MODE:-table}"

    if [ ! -f "$CLIENT_REVERSE_FILE" ]; then
        if [ "$format" = "json" ]; then
            printf '[]\n'
        else
            printf 'No client reverse tunnels recorded.\n'
        fi
        return 0
    fi
    client_reverse_store_require "$CLIENT_REVERSE_FILE"
    client_reverse_store_print_table "$CLIENT_REVERSE_FILE"
}

cmd_add() {
    subnet_arg=""
    server_arg=""
    tunnel_override=""
    outbound_arg=""

    while [ "$#" -gt 0 ]; do
        case "$1" in
            -h|--help)
                usage 0
                ;;
            --subnet)
                if [ "$#" -lt 2 ]; then
                    printf 'Option %s requires an argument.\n' "$1" >&2
                    usage 1
                fi
                subnet_arg=$(client_reverse_trim_spaces "$2")
                if [ -n "$subnet_arg" ] && ! validate_subnet "$subnet_arg"; then
                    xray_die "Invalid subnet: $subnet_arg"
                fi
                shift
                ;;
            --subnet=*)
                subnet_arg=$(client_reverse_trim_spaces "${1#*=}")
                if [ -n "$subnet_arg" ] && ! validate_subnet "$subnet_arg"; then
                    xray_die "Invalid subnet: $subnet_arg"
                fi
                ;;
            --server)
                if [ "$#" -lt 2 ]; then
                    printf 'Option %s requires an argument.\n' "$1" >&2
                    usage 1
                fi
                server_arg=$(client_reverse_trim_spaces "$2")
                shift
                ;;
            --server=*)
                server_arg=$(client_reverse_trim_spaces "${1#*=}")
                ;;
            --id)
                if [ "$#" -lt 2 ]; then
                    printf 'Option %s requires an argument.\n' "$1" >&2
                    usage 1
                fi
                tunnel_override=$(client_reverse_trim_spaces "$2")
                shift
                ;;
            --id=*)
                tunnel_override=$(client_reverse_trim_spaces "${1#*=}")
                ;;
            --outbound)
                if [ "$#" -lt 2 ]; then
                    printf 'Option %s requires an argument.\n' "$1" >&2
                    usage 1
                fi
                outbound_arg=$(client_reverse_trim_spaces "$2")
                shift
                ;;
            --outbound=*)
                outbound_arg=$(client_reverse_trim_spaces "${1#*=}")
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

    server_id=$(client_reverse_read_server "$server_arg")
    client_reverse_validate_server "$server_id"

    tunnel_id=$(reverse_resolve_tunnel_id "$subnet_arg" "$server_id" "$tunnel_override")

    suffix="${XRAY_REVERSE_SUFFIX:-.rev}"
    domain="$tunnel_id$suffix"
    tag="$domain"

    if [ -f "$CLIENT_REVERSE_FILE" ] && client_reverse_store_has "$CLIENT_REVERSE_FILE" "$tunnel_id"; then
        xray_log "Client reverse '$tunnel_id' already configured; skipping."
        return 0
    fi

    outbound_tag=$(resolve_client_outbound_tag "$server_id" "$outbound_arg")

    client_reverse_ensure_routing_file "$ROUTING_FILE" "$ROUTING_TEMPLATE_LOCAL" "$ROUTING_TEMPLATE_REMOTE"
    client_reverse_update_routing "$ROUTING_FILE" "$tunnel_id" "$suffix" "$outbound_tag"
    client_reverse_store_add "$CLIENT_REVERSE_FILE" "$CLIENT_REVERSE_DIR" "$tunnel_id" "$domain" "$tag" "$server_id" "$outbound_tag"

    xray_restart_service "xray-p2p" "/etc/init.d/xray-p2p" ""
    xray_log "Client reverse '$tunnel_id' recorded (server $server_id, domain $domain)."
}

cmd_remove() {
    id_arg=""
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
                id_arg=$(client_reverse_trim_spaces "$2")
                shift
                ;;
            --id=*)
                id_arg=$(client_reverse_trim_spaces "${1#*=}")
                ;;
            --server)
                if [ "$#" -lt 2 ]; then
                    printf 'Option %s requires an argument.\n' "$1" >&2
                    usage 1
                fi
                server_arg=$(client_reverse_trim_spaces "$2")
                shift
                ;;
            --server=*)
                server_arg=$(client_reverse_trim_spaces "${1#*=}")
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

    if [ -n "$server_arg" ]; then
        client_reverse_validate_server "$server_arg"
    fi

    client_reverse_store_require "$CLIENT_REVERSE_FILE"

    if [ -n "$id_arg" ]; then
        tunnel_id=$(reverse_resolve_tunnel_id "" "" "$id_arg")
        if ! client_reverse_store_has "$CLIENT_REVERSE_FILE" "$tunnel_id"; then
            xray_die "Client reverse '$tunnel_id' not found in $CLIENT_REVERSE_FILE"
        fi
    else
        tunnel_id=$(client_reverse_choose_tunnel "$server_arg")
    fi

    entry=$(client_reverse_fetch_entry "$tunnel_id")
    if [ -z "$entry" ]; then
        xray_die "Client reverse '$tunnel_id' not found in $CLIENT_REVERSE_FILE"
    fi

    IFS='\t' read -r entry_id entry_server entry_outbound entry_domain <<EOF
$entry
EOF

    suffix="${XRAY_REVERSE_SUFFIX:-.rev}"
    client_reverse_store_remove "$CLIENT_REVERSE_FILE" "$tunnel_id"
    client_reverse_remove_routing "$ROUTING_FILE" "$tunnel_id" "$suffix"

    xray_restart_service "xray-p2p" "/etc/init.d/xray-p2p" ""
    xray_log "Client reverse '$entry_id' removed (server ${entry_server:-"-"}, outbound ${entry_outbound:-"-"}, domain ${entry_domain:-"-"})."
}

main() {
    while [ "$#" -gt 0 ]; do
        if xray_consume_json_flag "$@"; then
            shift "$XRAY_JSON_FLAG_CONSUMED"
            continue
        fi
        break
    done

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
            while [ "$#" -gt 0 ]; do
                if xray_consume_json_flag "$@"; then
                    shift "$XRAY_JSON_FLAG_CONSUMED"
                    continue
                fi
                break
            done
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
