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

DNS_FORWARD_CORE_LOCAL="${XRAY_DNS_FORWARD_CORE_LIB:-scripts/lib/dns_forward_core.sh}"
DNS_FORWARD_CORE_REMOTE="${XRAY_DNS_FORWARD_CORE_REMOTE:-scripts/lib/dns_forward_core.sh}"

dns_forward_load_repo_lib "$DNS_FORWARD_CORE_LOCAL" "$DNS_FORWARD_CORE_REMOTE"

dns_forward_init

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
