#!/bin/sh

set -eu

SCRIPT_NAME=${0##*/}

usage() {
    cat <<EOF
Usage:
  $SCRIPT_NAME                 List recorded DNS forwards.
  $SCRIPT_NAME list            Same as default list action.
  $SCRIPT_NAME add [DOMAIN_MASK] [DNS_IP]
  $SCRIPT_NAME remove DOMAIN_MASK

Environment:
  XRAY_CONFIG_DIR                  XRAY configuration directory (default: /etc/xray-p2p).
  XRAY_DNS_INBOUND_FILE            Override inbounds.json path.
  XRAY_INBOUND_FILE                Legacy override for inbounds.json path.
  XRAY_DNS_FORWARD_DIR             Metadata directory (default: \$XRAY_CONFIG_DIR/config).
  XRAY_DNS_FORWARD_FILE            Metadata file path (default: \$XRAY_DNS_FORWARD_DIR/dns_forwards.json).
  XRAY_DNS_FORWARD_LISTEN          Listener address (default: 127.0.0.1).
  XRAY_DNS_FORWARD_BASE_PORT       First port to probe (default: 53331).
  XRAY_DNS_FORWARD_DNSMASQ_SECTION dnsmasq UCI section (default: dhcp.@dnsmasq[0]).
  XRAY_DNS_FORWARD_DNSMASQ_SERVICE dnsmasq init script (default: /etc/init.d/dnsmasq).
  XRAY_DNS_FORWARD_XRAY_SERVICE    xray init script (default: /etc/init.d/xray-p2p).
  XRAY_DNS_FORWARD_REMARK_PREFIX   Remark prefix inside inbounds.json (default: dns-forward:).
  XRAY_DNS_FORWARD_TAG_PREFIX      Tag prefix for dokodemo-door inbounds (default: in_dns_).
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

DNS_FORWARD_LIB_TMP=""
DNS_FORWARD_TMP_FILES=""

dns_forward_register_tmp() {
    tmp_file="$1"
    if [ -n "$tmp_file" ]; then
        DNS_FORWARD_TMP_FILES="${DNS_FORWARD_TMP_FILES} $tmp_file"
    fi
}

dns_forward_exit_cleanup() {
    set +e
    for tmp in $DNS_FORWARD_TMP_FILES; do
        if [ -n "$tmp" ]; then
            rm -f "$tmp" 2>/dev/null
        fi
    done
    for tmp in $DNS_FORWARD_LIB_TMP; do
        if [ -n "$tmp" ]; then
            rm -f "$tmp" 2>/dev/null
        fi
    done
}

trap dns_forward_exit_cleanup EXIT
trap 'dns_forward_exit_cleanup; exit 1' INT TERM HUP

dns_forward_load_repo_lib() {
    local_spec="$1"
    remote_spec="$2"
    resolved=""
    tmp=""

    if resolved=$(xray_resolve_local_path "$local_spec" 2>/dev/null) && [ -r "$resolved" ]; then
        # shellcheck disable=SC1090
        . "$resolved"
        return 0
    fi

    tmp="$(xray_fetch_repo_script "$remote_spec")" || xray_die "Required library not available: $remote_spec"
    DNS_FORWARD_LIB_TMP="${DNS_FORWARD_LIB_TMP} $tmp"
    # shellcheck disable=SC1090
    . "$tmp"
}

DNS_FORWARD_STORE_LOCAL="${XRAY_DNS_FORWARD_STORE_LIB:-lib/dns_forward_store.sh}"
DNS_FORWARD_STORE_REMOTE="${XRAY_DNS_FORWARD_STORE_REMOTE:-scripts/lib/dns_forward_store.sh}"

dns_forward_load_repo_lib "$DNS_FORWARD_STORE_LOCAL" "$DNS_FORWARD_STORE_REMOTE"

XRAY_CONFIG_DIR=${XRAY_CONFIG_DIR:-/etc/xray-p2p}
XRAY_INBOUND_FILE_DEFAULT="${XRAY_CONFIG_DIR%/}/inbounds.json"
XRAY_INBOUND_FILE=${XRAY_DNS_INBOUND_FILE:-${XRAY_INBOUND_FILE:-$XRAY_INBOUND_FILE_DEFAULT}}

XRAY_DNS_FORWARD_DIR_DEFAULT="${XRAY_CONFIG_DIR%/}/config"
XRAY_DNS_FORWARD_DIR=${XRAY_DNS_FORWARD_DIR:-$XRAY_DNS_FORWARD_DIR_DEFAULT}
XRAY_DNS_FORWARD_FILE_DEFAULT="${XRAY_DNS_FORWARD_DIR%/}/dns_forwards.json"
XRAY_DNS_FORWARD_FILE=${XRAY_DNS_FORWARD_FILE:-$XRAY_DNS_FORWARD_FILE_DEFAULT}

LISTEN_ADDRESS=${XRAY_DNS_FORWARD_LISTEN:-127.0.0.1}
BASE_LOCAL_PORT=${XRAY_DNS_FORWARD_BASE_PORT:-53331}
DNSMASQ_SECTION=${XRAY_DNS_FORWARD_DNSMASQ_SECTION:-dhcp.@dnsmasq[0]}
DNSMASQ_SERVICE=${XRAY_DNS_FORWARD_DNSMASQ_SERVICE:-/etc/init.d/dnsmasq}
XRAY_SERVICE=${XRAY_DNS_FORWARD_XRAY_SERVICE:-/etc/init.d/xray-p2p}
DNS_REMARK_PREFIX=${XRAY_DNS_FORWARD_REMARK_PREFIX:-dns-forward:}
DNS_TAG_PREFIX=${XRAY_DNS_FORWARD_TAG_PREFIX:-in_dns_}

dns_forward_trim() {
    printf '%s' "$1" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//'
}

dns_forward_validate_domain_mask() {
    case "$1" in
        ''|*[!A-Za-z0-9.*-]*) return 1 ;;
    esac
    case "$1" in
        *.*) return 0 ;;
    esac
    return 1
}

dns_forward_base_domain() {
    mask="$1"
    case "$mask" in
        '*.'*) base=${mask#*.} ;;
        .*) base=${mask#.} ;;
        *) base=$mask ;;
    esac
    base=${base#.}
    printf '%s' "$base"
}

dns_forward_prompt() {
    prompt="$1"
    response=""
    if [ -t 0 ]; then
        printf '%s' "$prompt"
        IFS= read -r response
    elif [ -r /dev/tty ]; then
        printf '%s' "$prompt"
        IFS= read -r response </dev/tty
    else
        response=""
    fi
    printf '%s' "$response"
}

dns_forward_require_inbound_file() {
    if [ ! -f "$XRAY_INBOUND_FILE" ]; then
        xray_die "XRAY inbound file $XRAY_INBOUND_FILE not found"
    fi
}

dns_forward_require_dns_tools() {
    xray_require_cmd jq
    xray_require_cmd uci
    xray_require_cmd sed
    xray_require_cmd cmp
    xray_require_cmd sort
    xray_require_cmd grep
}

cmd_list() {
    if [ ! -f "$XRAY_DNS_FORWARD_FILE" ]; then
        xray_log "No DNS forwards recorded (metadata file not found at $XRAY_DNS_FORWARD_FILE)."
        return 0
    fi

    if ! jq -e 'length > 0' "$XRAY_DNS_FORWARD_FILE" >/dev/null 2>&1; then
        xray_log "No DNS forwards recorded."
        return 0
    fi

    dns_forward_store_print_table "$XRAY_DNS_FORWARD_FILE"
}

cmd_add() {
    dns_forward_require_dns_tools
    dns_forward_require_inbound_file

    domain_mask_arg=""
    dns_ip_arg=""

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
                if [ -z "$domain_mask_arg" ]; then
                    domain_mask_arg="$1"
                elif [ -z "$dns_ip_arg" ]; then
                    dns_ip_arg="$1"
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

    domain_mask=$(dns_forward_trim "$domain_mask_arg")
    if [ -z "$domain_mask" ]; then
        domain_mask=$(dns_forward_prompt 'Enter domain mask (e.g. *.corp.test.com): ')
    fi
    domain_mask=$(dns_forward_trim "$domain_mask")

    if ! dns_forward_validate_domain_mask "$domain_mask"; then
        xray_die "Domain mask must contain only letters, digits, dots, dashes, or asterisks (example: *.corp.test.com)"
    fi

    dns_ip=$(dns_forward_trim "$dns_ip_arg")
    if [ -z "$dns_ip" ]; then
        dns_ip=$(dns_forward_prompt 'Enter upstream DNS IP: ')
    fi
    dns_ip=$(dns_forward_trim "$dns_ip")

    if ! validate_ipv4 "$dns_ip"; then
        xray_die "Upstream DNS IP must be a valid IPv4 address"
    fi

    base_domain=$(dns_forward_base_domain "$domain_mask")
    if [ -z "$base_domain" ] || printf '%s' "$base_domain" | grep '\*' >/dev/null 2>&1; then
        xray_die "Cannot derive base domain from mask '$domain_mask'"
    fi

    remark="$DNS_REMARK_PREFIX$domain_mask"
    rebind_value="$base_domain"

    existing_entry=$(jq -r --arg remark "$remark" '
        (.inbounds[]? | select((.remark // "") == $remark) | [(.tag // ""), (.port // empty)] | @tsv)
    ' "$XRAY_INBOUND_FILE" | head -n 1)

    existing_tag=""
    existing_port=""
    if [ -n "$existing_entry" ]; then
        set -- $existing_entry
        existing_tag="${1:-}"
        existing_port="${2:-}"
    fi

    all_ports=$(jq -r '.inbounds[]? | .port | select(type == "number")' "$XRAY_INBOUND_FILE" 2>/dev/null | sort -n)
    existing_tags=$(jq -r '.inbounds[]? | .tag // empty' "$XRAY_INBOUND_FILE" 2>/dev/null)

    if [ -n "$existing_port" ]; then
        local_port="$existing_port"
    else
        candidate=$BASE_LOCAL_PORT
        while echo "$all_ports" | grep -qx "$candidate"; do
            candidate=$((candidate + 1))
            if [ "$candidate" -gt 65535 ]; then
                xray_die "No free ports available from $BASE_LOCAL_PORT to 65535"
            fi
        done
        local_port="$candidate"
    fi

    if [ -n "$existing_tag" ]; then
        tag="$existing_tag"
    else
        base_tag="$DNS_TAG_PREFIX$(printf '%s' "$domain_mask" | tr 'A-Z' 'a-z' | sed 's/[^0-9a-z]/_/g')"
        tag="$base_tag"
        suffix=1
        while echo "$existing_tags" | grep -Fx "$tag" >/dev/null 2>&1; do
            tag="${base_tag}_${suffix}"
            suffix=$((suffix + 1))
        done
    fi

    if [ -n "$existing_port" ]; then
        xray_log "Updating DNS forwarding for $domain_mask (port $local_port)"
    else
        xray_log "Adding DNS forwarding for $domain_mask on port $local_port"
    fi

    xray_log "Upstream DNS IP: $dns_ip"

    tmp_inbound=$(mktemp 2>/dev/null)
    if [ -z "$tmp_inbound" ]; then
        xray_die "Unable to create temporary file"
    fi
    dns_forward_register_tmp "$tmp_inbound"

    if ! jq --arg listen "$LISTEN_ADDRESS" \
            --arg remark "$remark" \
            --arg tag "$tag" \
            --arg address "$dns_ip" \
            --argjson port "$local_port" '
        def dnsInbound:
            {
                tag: $tag,
                remark: $remark,
                listen: $listen,
                port: $port,
                protocol: "dokodemo-door",
                settings: {
                    address: $address,
                    port: 53,
                    network: "tcp,udp",
                    followRedirect: false
                }
            };
        if (.inbounds | type) != "array" then
            error("inbounds must be an array")
        else
            (reduce .inbounds[]? as $item
                ({items: [], found: false};
                    if ($item.remark // "") == $remark then
                        if .found then
                            {items: .items, found: true}
                        else
                            {items: .items + [dnsInbound], found: true}
                        end
                    else
                        {items: .items + [$item], found: .found}
                    end
                )) as $state
            | if $state.found then
                .inbounds = $state.items
              else
                .inbounds = ($state.items + [dnsInbound])
              end
        end
    ' "$XRAY_INBOUND_FILE" >"$tmp_inbound"; then
        xray_die "Failed to update $XRAY_INBOUND_FILE"
    fi

    inbound_changed=0
    if ! cmp -s "$tmp_inbound" "$XRAY_INBOUND_FILE"; then
        chmod 0644 "$tmp_inbound" 2>/dev/null || true
        mv "$tmp_inbound" "$XRAY_INBOUND_FILE"
        inbound_changed=1
        xray_log "Updated $XRAY_INBOUND_FILE"
    else
        xray_log "No changes required in $XRAY_INBOUND_FILE"
    fi

    server_value="/$domain_mask/$LISTEN_ADDRESS#$local_port"
    rebind_value="$base_domain"

    uci_changed=0
    servers_output=$(uci -q show "$DNSMASQ_SECTION.server" 2>/dev/null || true)
    server_present=0

    if [ -n "$servers_output" ]; then
        while IFS= read -r line; do
            value=${line#*=}
            value=${value%\'}
            value=${value#\'}
            value=${value%\"}
            value=${value#\"}
            case "$value" in
                "/$domain_mask/"*)
                    if [ "$value" = "$server_value" ]; then
                        if [ "$server_present" -eq 0 ]; then
                            server_present=1
                        else
                            uci del_list "$DNSMASQ_SECTION.server=$value"
                            uci_changed=1
                        fi
                    else
                        uci del_list "$DNSMASQ_SECTION.server=$value"
                        uci_changed=1
                    fi
                    ;;
            esac
        done <<EOF
$servers_output
EOF
    fi

    if [ "$server_present" -eq 0 ]; then
        uci add_list "$DNSMASQ_SECTION.server"="$server_value"
        uci_changed=1
        xray_log "Set dnsmasq server entry: $server_value"
    else
        xray_log "dnsmasq server entry already set"
    fi

    rebind_output=$(uci -q show "$DNSMASQ_SECTION.rebind_domain" 2>/dev/null || true)
    rebind_present=0

    if [ -n "$rebind_output" ]; then
        while IFS= read -r line; do
            value=${line#*=}
            value=${value%\'}
            value=${value#\'}
            value=${value%\"}
            value=${value#\"}
            if [ "$value" = "$rebind_value" ]; then
                if [ "$rebind_present" -eq 0 ]; then
                    rebind_present=1
                else
                    uci del_list "$DNSMASQ_SECTION.rebind_domain=$value"
                    uci_changed=1
                fi
            fi
        done <<EOF
$rebind_output
EOF
    fi

    if [ "$rebind_present" -eq 0 ]; then
        uci add_list "$DNSMASQ_SECTION.rebind_domain"="$rebind_value"
        uci_changed=1
        xray_log "Set dnsmasq rebind_domain entry: $rebind_value"
    else
        xray_log "dnsmasq rebind_domain entry already set"
    fi

    if [ "$uci_changed" -eq 1 ]; then
        uci commit dhcp
        if [ -x "$DNSMASQ_SERVICE" ]; then
            if "$DNSMASQ_SERVICE" restart >/dev/null 2>&1; then
                xray_log "dnsmasq restarted"
            else
                xray_log "dnsmasq restart failed; please restart manually"
            fi
        else
            xray_log "dnsmasq service script not found at $DNSMASQ_SERVICE"
        fi
    else
        xray_log "dnsmasq configuration already up to date"
    fi

    if [ "$inbound_changed" -eq 1 ]; then
        if [ -x "$XRAY_SERVICE" ]; then
            xray_restart_service "xray-p2p" "$XRAY_SERVICE"
            if [ "${XRAY_SKIP_RESTART:-0}" != "1" ]; then
                xray_log "xray-p2p restarted"
            fi
        else
            xray_log "xray-p2p service script not found at $XRAY_SERVICE"
        fi
    else
        xray_log "xray-p2p configuration already up to date"
    fi

    dns_forward_store_add "$XRAY_DNS_FORWARD_FILE" "$XRAY_DNS_FORWARD_DIR" "$domain_mask" "$dns_ip" "$LISTEN_ADDRESS" "$local_port" "$tag" "$remark" "$rebind_value"

    xray_log "Forwarding ready: $domain_mask -> $dns_ip via $LISTEN_ADDRESS#$local_port"
}

cmd_remove() {
    if [ "$#" -ne 1 ]; then
        printf 'remove command expects exactly one DOMAIN_MASK argument.\n' >&2
        usage 1
    fi

    dns_forward_require_dns_tools
    dns_forward_require_inbound_file

    domain_mask=$(dns_forward_trim "$1")
    if [ -z "$domain_mask" ]; then
        xray_die "Domain mask cannot be empty."
    fi

    if ! dns_forward_validate_domain_mask "$domain_mask"; then
        xray_die "Invalid domain mask: $domain_mask"
    fi

    if [ ! -f "$XRAY_DNS_FORWARD_FILE" ]; then
        xray_die "DNS forward metadata file not found: $XRAY_DNS_FORWARD_FILE"
    fi

    if ! record=$(dns_forward_store_get "$XRAY_DNS_FORWARD_FILE" "$domain_mask"); then
        xray_die "DNS forward '$domain_mask' not found in $XRAY_DNS_FORWARD_FILE"
    fi

    listen=$(printf '%s\n' "$record" | jq -r '.listen // ""')
    if [ -z "$listen" ]; then
        listen="$LISTEN_ADDRESS"
    fi
    local_port=$(printf '%s\n' "$record" | jq -r '.local_port // empty')
    if [ -z "$local_port" ]; then
        xray_die "Metadata for '$domain_mask' is missing local_port."
    fi
    remark=$(printf '%s\n' "$record" | jq -r '.remark // ""')
    if [ -z "$remark" ]; then
        remark="$DNS_REMARK_PREFIX$domain_mask"
    fi
    rebind_value=$(printf '%s\n' "$record" | jq -r '.rebind // ""')
    if [ -z "$rebind_value" ]; then
        rebind_value=$(dns_forward_base_domain "$domain_mask")
    fi

    tmp_inbound=$(mktemp 2>/dev/null)
    if [ -z "$tmp_inbound" ]; then
        xray_die "Unable to create temporary file"
    fi
    dns_forward_register_tmp "$tmp_inbound"

    if ! jq --arg remark "$remark" '
        if (.inbounds | type) != "array" then
            error("inbounds must be an array")
        else
            .inbounds = [
                .inbounds[]?
                | select((.remark // "") != $remark)
            ]
        end
    ' "$XRAY_INBOUND_FILE" >"$tmp_inbound"; then
        xray_die "Failed to update $XRAY_INBOUND_FILE"
    fi

    inbound_changed=0
    if ! cmp -s "$tmp_inbound" "$XRAY_INBOUND_FILE"; then
        chmod 0644 "$tmp_inbound" 2>/dev/null || true
        mv "$tmp_inbound" "$XRAY_INBOUND_FILE"
        inbound_changed=1
        xray_log "Updated $XRAY_INBOUND_FILE"
    else
        xray_log "No changes required in $XRAY_INBOUND_FILE"
    fi

    server_value="/$domain_mask/$listen#$local_port"
    uci_changed=0

    servers_output=$(uci -q show "$DNSMASQ_SECTION.server" 2>/dev/null || true)
    if [ -n "$servers_output" ]; then
        while IFS= read -r line; do
            value=${line#*=}
            value=${value%\'}
            value=${value#\'}
            value=${value%\"}
            value=${value#\"}
            if [ "$value" = "$server_value" ]; then
                uci del_list "$DNSMASQ_SECTION.server=$value"
                uci_changed=1
            fi
        done <<EOF
$servers_output
EOF
    fi

    other_rebind_count=$(jq -r --arg rebind "$rebind_value" --arg domain_mask "$domain_mask" '
        [ .[]? | select((.rebind // "") == $rebind and (.domain_mask // "") != $domain_mask) ] | length
    ' "$XRAY_DNS_FORWARD_FILE" 2>/dev/null || printf '0')

    if [ "$other_rebind_count" -eq 0 ]; then
        rebind_output=$(uci -q show "$DNSMASQ_SECTION.rebind_domain" 2>/dev/null || true)
        if [ -n "$rebind_output" ]; then
            while IFS= read -r line; do
                value=${line#*=}
                value=${value%\'}
                value=${value#\'}
                value=${value%\"}
                value=${value#\"}
                if [ "$value" = "$rebind_value" ]; then
                    uci del_list "$DNSMASQ_SECTION.rebind_domain=$value"
                    uci_changed=1
                fi
            done <<EOF
$rebind_output
EOF
        fi
    fi

    if [ "$uci_changed" -eq 1 ]; then
        uci commit dhcp
        if [ -x "$DNSMASQ_SERVICE" ]; then
            if "$DNSMASQ_SERVICE" restart >/dev/null 2>&1; then
                xray_log "dnsmasq restarted"
            else
                xray_log "dnsmasq restart failed; please restart manually"
            fi
        else
            xray_log "dnsmasq service script not found at $DNSMASQ_SERVICE"
        fi
    else
        xray_log "dnsmasq configuration already up to date"
    fi

    if [ "$inbound_changed" -eq 1 ]; then
        if [ -x "$XRAY_SERVICE" ]; then
            xray_restart_service "xray-p2p" "$XRAY_SERVICE"
            if [ "${XRAY_SKIP_RESTART:-0}" != "1" ]; then
                xray_log "xray-p2p restarted"
            fi
        else
            xray_log "xray-p2p service script not found at $XRAY_SERVICE"
        fi
    else
        xray_log "xray-p2p configuration already up to date"
    fi

    dns_forward_store_remove "$XRAY_DNS_FORWARD_FILE" "$domain_mask"
    xray_log "DNS forward '$domain_mask' removed."
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
