#!/bin/sh
set -eu

SCRIPT_NAME=${0##*/}

usage() {
    cat <<EOF
Usage:
  $SCRIPT_NAME                 List configured client outbounds.
  $SCRIPT_NAME list            Same as default list action.
  $SCRIPT_NAME add TROJAN_URL [SUBNET]
  $SCRIPT_NAME remove TAG

Environment:
  XRAY_CONFIG_DIR       Override XRAY configuration directory (default: /etc/xray-p2p).
  XRAY_OUTBOUNDS_FILE   Override outbounds.json path.
  XRAY_ROUTING_FILE     Override routing.json path.
  XRAY_INBOUND_FILE     Override inbounds.json used for redirect port detection.
  XRAY_REDIRECT_SCRIPT  Override redirect script path (default: scripts/redirect.sh).
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

if ! xray_common_try_source \
    "${XRAY_CLIENT_CONNECTION_LIB:-scripts/lib/client_connection.sh}" \
    "scripts/lib/client_connection.sh" \
    "lib/client_connection.sh"; then
    xray_die "Unable to load client connection library."
fi

CLIENT_USER_CONFIG_DIR="${XRAY_CONFIG_DIR:-/etc/xray-p2p}"
CLIENT_USER_OUTBOUNDS_FILE="${XRAY_OUTBOUNDS_FILE:-$CLIENT_USER_CONFIG_DIR/outbounds.json}"
CLIENT_USER_ROUTING_FILE="${XRAY_ROUTING_FILE:-$CLIENT_USER_CONFIG_DIR/routing.json}"

if [ -n "${XRAY_INBOUND_FILE:-}" ]; then
    CLIENT_USER_INBOUND_FILE="$XRAY_INBOUND_FILE"
elif [ -n "${XRAY_DNS_INBOUND_FILE:-}" ]; then
    CLIENT_USER_INBOUND_FILE="$XRAY_DNS_INBOUND_FILE"
else
    CLIENT_USER_INBOUND_FILE="$CLIENT_USER_CONFIG_DIR/inbounds.json"
fi

CLIENT_USER_REDIRECT_SCRIPT="${XRAY_REDIRECT_SCRIPT:-scripts/redirect.sh}"

client_user_prepare_config() {
    if [ ! -f "$CLIENT_USER_OUTBOUNDS_FILE" ]; then
        xray_seed_file_from_template "$CLIENT_USER_OUTBOUNDS_FILE" "config_templates/client/outbounds.json"
    fi
    if [ ! -f "$CLIENT_USER_ROUTING_FILE" ]; then
        xray_seed_file_from_template "$CLIENT_USER_ROUTING_FILE" "config_templates/client/routing.json"
    fi
}

client_user_require_jq() {
    xray_require_cmd jq
}

client_user_list() {
    if [ ! -f "$CLIENT_USER_OUTBOUNDS_FILE" ]; then
        xray_log "Outbound configuration not found: $CLIENT_USER_OUTBOUNDS_FILE"
        return 0
    fi

    client_user_require_jq

    local entries
    entries="$(jq -r '
        (.outbounds // []) |
        map({
            tag: (.tag // "untagged"),
            protocol: (.protocol // ""),
            address: (.settings.servers[0].address // ""),
            port: ((.settings.servers[0].port // "") | tostring),
            network: (.streamSettings.network // ""),
            security: (.streamSettings.security // "")
        }) |
        .[]? |
        [ .tag, .protocol, (.address + ":" + .port), .network, .security ] |
        @tsv
    ' "$CLIENT_USER_OUTBOUNDS_FILE" 2>/dev/null || true)"

    if [ -z "${entries:-}" ]; then
        printf 'No client outbounds configured.\n'
        return 0
    fi

    printf 'Tag\tProtocol\tTarget\tNetwork\tSecurity\n'
    printf '%s\n' "$entries" | while IFS=$'\t' read -r tag protocol target network security; do
        printf '%s\t%s\t%s\t%s\t%s\n' "$tag" "$protocol" "$target" "$network" "$security"
    done
}

client_user_outbound_exists() {
    local tag="$1"
    jq -e --arg tag "$tag" '
        (.outbounds // []) | any(.[]?; (.tag // "") == $tag)
    ' "$CLIENT_USER_OUTBOUNDS_FILE" >/dev/null 2>&1
}

client_user_run_redirect() {
    local action="$1"
    shift
    XRAY_CONFIG_DIR="$CLIENT_USER_CONFIG_DIR" \
    XRAY_INBOUND_FILE="$CLIENT_USER_INBOUND_FILE" \
    xray_run_repo_script required "$CLIENT_USER_REDIRECT_SCRIPT" "scripts/redirect.sh" "$action" "$@"
}

client_user_add() {
    if [ "$#" -lt 1 ]; then
        usage 1
    fi

    local connection="$1"
    shift
    local subnet="${1:-}"

    client_user_prepare_config
    client_user_require_jq

    if [ ! -f "$CLIENT_USER_OUTBOUNDS_FILE" ]; then
        xray_die "Outbound configuration not found: $CLIENT_USER_OUTBOUNDS_FILE"
    fi

    client_connection_parse "$connection"
    local tag="$CLIENT_CONNECTION_TAG"

    if client_user_outbound_exists "$tag"; then
        xray_die "Outbound with tag '$tag' already exists."
    fi

    local allow_insecure_json="false"
    if [ "${CLIENT_CONNECTION_ALLOW_INSECURE:-false}" = "true" ]; then
        allow_insecure_json="true"
    fi

    local tmp_out
    tmp_out="$(mktemp 2>/dev/null)" || xray_die "Unable to create temporary file for outbounds update"

    if ! jq \
        --arg tag "$tag" \
        --arg password "$CLIENT_CONNECTION_PASSWORD" \
        --arg address "$CLIENT_CONNECTION_HOST" \
        --arg serverName "$CLIENT_CONNECTION_SERVER_NAME" \
        --arg network "$CLIENT_CONNECTION_NETWORK" \
        --arg security "$CLIENT_CONNECTION_SECURITY" \
        --argjson port "$CLIENT_CONNECTION_PORT" \
        --argjson allowInsecure "$allow_insecure_json" \
        '
        (.outbounds //= [])
        | .outbounds += [{
            protocol: "trojan",
            settings: {
                servers: [
                    {
                        address: $address,
                        port: $port,
                        password: $password
                    }
                ]
            },
            streamSettings: {
                network: $network,
                security: $security,
                tlsSettings: {
                    allowInsecure: $allowInsecure,
                    serverName: $serverName
                }
            },
            tag: $tag
        }]
        ' "$CLIENT_USER_OUTBOUNDS_FILE" >"$tmp_out"; then
        rm -f "$tmp_out"
        xray_die "Failed to update $CLIENT_USER_OUTBOUNDS_FILE"
    fi

    mv "$tmp_out" "$CLIENT_USER_OUTBOUNDS_FILE"

    xray_log "Added outbound '$tag' -> ${CLIENT_CONNECTION_HOST}:${CLIENT_CONNECTION_PORT}"

    if [ -n "$subnet" ]; then
        if ! validate_subnet "$subnet"; then
            xray_die "Subnet must be a valid IPv4 CIDR (example: 10.0.101.0/24)"
        fi

        if [ ! -f "$CLIENT_USER_ROUTING_FILE" ]; then
            xray_die "Routing configuration not found: $CLIENT_USER_ROUTING_FILE"
        fi

        if jq -e \
            --arg tag "$tag" \
            --arg subnet "$subnet" \
            '
            (.routing.rules // []) | any(.[]?; (.outboundTag // "") == $tag and (.ip // []) | index($subnet))
            ' "$CLIENT_USER_ROUTING_FILE" >/dev/null 2>&1; then
            xray_die "Routing rule for subnet $subnet and tag $tag already exists."
        fi

        local tmp_route
        tmp_route="$(mktemp 2>/dev/null)" || xray_die "Unable to create temporary file for routing update"

        if ! jq \
            --arg tag "$tag" \
            --arg subnet "$subnet" \
            '
            (.routing //= {})
            | (.routing.rules //= [])
            | .routing.rules += [{
                type: "field",
                ip: [$subnet],
                outboundTag: $tag
            }]
            ' "$CLIENT_USER_ROUTING_FILE" >"$tmp_route"; then
            rm -f "$tmp_route"
            xray_die "Failed to update $CLIENT_USER_ROUTING_FILE"
        fi

        mv "$tmp_route" "$CLIENT_USER_ROUTING_FILE"
        xray_log "Added routing rule for subnet $subnet -> $tag"

        client_user_run_redirect add "$subnet"
    fi
}

client_user_collect_subnets() {
    local tag="$1"
    if [ ! -f "$CLIENT_USER_ROUTING_FILE" ]; then
        return 0
    fi

    jq -r --arg tag "$tag" '
        (.routing.rules // []) |
        map(select((.outboundTag // "") == $tag)) |
        map(.ip // []) |
        add // [] |
        unique |
        .[]
    ' "$CLIENT_USER_ROUTING_FILE" 2>/dev/null
}

client_user_remove() {
    if [ "$#" -lt 1 ]; then
        usage 1
    fi

    local tag="$1"

    if [ ! -f "$CLIENT_USER_OUTBOUNDS_FILE" ]; then
        xray_die "Outbound configuration not found: $CLIENT_USER_OUTBOUNDS_FILE"
    fi

    client_user_require_jq

    if ! client_user_outbound_exists "$tag"; then
        xray_die "Outbound with tag '$tag' not found."
    fi

    local subnets
    subnets="$(client_user_collect_subnets "$tag" || true)"

    local tmp_out
    tmp_out="$(mktemp 2>/dev/null)" || xray_die "Unable to create temporary file for outbounds update"

    if ! jq --arg tag "$tag" '
        (.outbounds //= [])
        | .outbounds = [ .outbounds[] | select((.tag // "") != $tag) ]
        ' "$CLIENT_USER_OUTBOUNDS_FILE" >"$tmp_out"; then
        rm -f "$tmp_out"
        xray_die "Failed to update $CLIENT_USER_OUTBOUNDS_FILE"
    fi

    mv "$tmp_out" "$CLIENT_USER_OUTBOUNDS_FILE"

    xray_log "Removed outbound '$tag'"

    if [ -f "$CLIENT_USER_ROUTING_FILE" ]; then
        local tmp_route
        tmp_route="$(mktemp 2>/dev/null)" || xray_die "Unable to create temporary file for routing update"

        if ! jq --arg tag "$tag" '
            (.routing //= {})
            | (.routing.rules //= [])
            | .routing.rules = [ .routing.rules[] | select((.outboundTag // "") != $tag) ]
            ' "$CLIENT_USER_ROUTING_FILE" >"$tmp_route"; then
            rm -f "$tmp_route"
            xray_die "Failed to update $CLIENT_USER_ROUTING_FILE"
        fi

        mv "$tmp_route" "$CLIENT_USER_ROUTING_FILE"
        xray_log "Removed routing rules referencing tag $tag"
    fi

    if [ -n "${subnets:-}" ]; then
        printf '%s\n' "$subnets" | while IFS= read -r subnet; do
            [ -n "$subnet" ] || continue
            client_user_run_redirect remove "$subnet"
        done
    fi
}

main() {
    umask 077

    if [ "$#" -eq 0 ]; then
        client_user_list
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
            client_user_list
            ;;
        add)
            client_user_add "$@"
            ;;
        remove)
            client_user_remove "$@"
            ;;
        *)
            printf 'Unknown command: %s\n' "$command" >&2
            usage 1
            ;;
    esac
}

main "$@"
