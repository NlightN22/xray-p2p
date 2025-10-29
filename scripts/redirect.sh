#!/bin/sh

set -eu

SCRIPT_NAME=${0##*/}

usage() {
    cat <<EOF
Usage:
  $SCRIPT_NAME                 List configured transparent redirects.
  $SCRIPT_NAME list            Same as default list action.
  $SCRIPT_NAME add [SUBNET] [PORT]
  $SCRIPT_NAME remove [SUBNET|--all]

Environment:
  NFT_SNIPPET        Override nftables snippet path (default: /etc/nftables.d/xray-transparent.nft).
  NFT_SNIPPET_DIR    Override entry directory (default: /etc/nftables.d/xray-transparent.d).
  XRAY_CONFIG_DIR    XRAY configuration directory (default: /etc/xray-p2p).
  XRAY_INBOUND_FILE  Path to inbounds.json used to detect dokodemo-door ports.
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

NFT_SNIPPET="${NFT_SNIPPET:-/etc/nftables.d/xray-transparent.nft}"
NFT_SNIPPET_DIR="${NFT_SNIPPET_DIR:-/etc/nftables.d/xray-transparent.d}"
XRAY_CONFIG_DIR="${XRAY_CONFIG_DIR:-/etc/xray-p2p}"

if [ -z "${XRAY_INBOUND_FILE:-}" ]; then
    if [ -n "${XRAY_DNS_INBOUND_FILE:-}" ]; then
        XRAY_INBOUND_FILE="$XRAY_DNS_INBOUND_FILE"
    else
        XRAY_INBOUND_FILE="$XRAY_CONFIG_DIR/inbounds.json"
    fi
fi

if ! xray_common_try_source \
    "${XRAY_REDIRECT_LIB:-}" \
    "scripts/lib/redirect.sh" \
    "lib/redirect.sh"; then
    tmp_lib="$(xray_fetch_repo_script "scripts/lib/redirect.sh")" || xray_die "Unable to load redirect library"
    # shellcheck disable=SC1090
    . "$tmp_lib"
    rm -f "$tmp_lib"
fi

redirect_prompt_subnet() {
    local subnet
    if [ -t 0 ]; then
        printf 'Enter destination subnet (CIDR, e.g. 10.0.101.0/24): '
        IFS= read -r subnet
    elif [ -r /dev/tty ]; then
        printf 'Enter destination subnet (CIDR, e.g. 10.0.101.0/24): ' >/dev/tty
        IFS= read -r subnet </dev/tty
    else
        xray_die "Subnet argument is required"
    fi
    printf '%s' "$subnet"
}

cmd_list() {
    local entries
    redirect_migrate_legacy_snippet || true
    entries="$(redirect_find_entries || true)"
    if [ -z "${entries:-}" ]; then
        printf 'No transparent redirect entries found.\n'
        return
    fi
    {
        printf 'Subnet\tPort\n'
        printf '%s\n' "$entries" | while IFS= read -r entry; do
            [ -n "$entry" ] || continue
            SUBNET=''
            PORT=''
            # shellcheck disable=SC1090
            . "$entry"
            if [ -n "$SUBNET" ] && [ -n "$PORT" ]; then
                printf '%s\t%s\n' "$SUBNET" "$PORT"
            fi
        done
    } | xray_print_table
}

cmd_add() {
    local subnet port_arg ports count idx answer port entry_file
    subnet="${1:-}"
    port_arg="${2:-}"
    if [ -n "$subnet" ]; then
        shift
    else
        subnet=$(redirect_prompt_subnet)
    fi
    if [ -n "${port_arg:-}" ]; then
        shift
    fi
    if ! validate_subnet "$subnet"; then
        xray_die "Subnet must be a valid IPv4 CIDR (example: 10.0.101.0/24 with prefix between 0 and 32)"
    fi
    if [ -n "${port_arg:-}" ]; then
        case "$port_arg" in
            *[!0-9]*)
                xray_die "Port must be a positive integer"
                ;;
        esac
        port="$port_arg"
    else
        ports=$(redirect_select_port || true)
        if [ -z "$ports" ]; then
            xray_die "No dokodemo-door inbounds found in $XRAY_INBOUND_FILE"
        fi
        count=$(printf '%s\n' "$ports" | grep -c '.')
        if [ "$count" -eq 1 ]; then
            port="$ports"
        else
            xray_log "Multiple dokodemo-door ports detected:"
            idx=1
            printf '%s\n' "$ports" | while IFS= read -r p; do
                printf ' [%d] %s\n' "$idx" "$p"
                idx=$((idx + 1))
            done
            if [ -t 0 ]; then
                printf 'Select port number: '
                read -r answer
            elif [ -r /dev/tty ]; then
                printf 'Select port number: '
                read -r answer </dev/tty
            else
                xray_die "Multiple dokodemo-door ports found but no interactive input available"
            fi
            case "$answer" in
                *[!0-9]*)
                    xray_die "Invalid selection"
                    ;;
            esac
            if [ "$answer" -lt 1 ] || [ "$answer" -gt "$count" ]; then
                xray_die "Selection out of range"
            fi
            port=$(printf '%s\n' "$ports" | sed -n "${answer}p")
        fi
    fi
    redirect_migrate_legacy_snippet || true
    entry_file=$(redirect_write_entry "$subnet" "$port")
    redirect_generate_snippet || true
    redirect_apply_rules
    xray_log "Transparent redirect active for subnet $subnet"
    xray_log "Dokodemo-door port: $port"
    xray_log "Snippet file: $NFT_SNIPPET"
    xray_log "Entry file: $entry_file"
}

cmd_remove() {
    local target mode entry_path
    target="${1:-}"
    if [ -z "$target" ]; then
        xray_die "remove command requires a subnet or --all"
    fi
    if [ "$target" = "--all" ]; then
        mode="all"
    else
        mode="single"
        if ! validate_subnet "$target"; then
            xray_die "Subnet must be a valid IPv4 CIDR (example: 10.0.101.0/24 with prefix between 0 and 32)"
        fi
    fi
    redirect_migrate_legacy_snippet || true
    if [ "$mode" = "all" ]; then
        if [ -d "$NFT_SNIPPET_DIR" ]; then
            find "$NFT_SNIPPET_DIR" -maxdepth 1 -type f -name '*.entry' -print 2>/dev/null | while IFS= read -r file; do
                [ -n "$file" ] || continue
                rm -f "$file"
                xray_log "Removed entry $file"
            done
        fi
        rm -f "$NFT_SNIPPET"
        xray_log "Removed nftables snippet $NFT_SNIPPET"
    else
        entry_path=$(redirect_entry_path_for_subnet "$target")
        if [ -f "$entry_path" ]; then
            rm -f "$entry_path"
            xray_log "Removed entry $entry_path"
        else
            xray_log "Entry for subnet $target not present"
        fi
    fi
    redirect_generate_snippet || true
    redirect_apply_rules
    xray_log "Transparent redirect rules updated"
}

main() {
    local command
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
