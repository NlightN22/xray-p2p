#!/bin/sh

set -eu

XRAY_INBOUND_FILE="/etc/xray-p2p/inbounds.json"
DNSMASQ_SECTION="dhcp.@dnsmasq[0]"
DNSMASQ_SERVICE="/etc/init.d/dnsmasq"
XRAY_SERVICE="/etc/init.d/xray-p2p"
DNS_REMARK_PREFIX="dns-forward:"

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

trim() {
    printf '%s' "$1" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//'
}

validate_domain_mask() {
    case "$1" in
        ''|*[!A-Za-z0-9.*-]*) return 1 ;;
    esac
    case "$1" in
        *.*) return 0 ;;
    esac
    return 1
}

derive_base_domain() {
    local mask="$1" base
    case "$mask" in
        '*.'*) base=${mask#*.} ;;
        .*) base=${mask#.} ;;
        *) base=$mask ;;
    esac
    base=${base#.}
    printf '%s' "$base"
}

collect_forwardings() {
    jq -r --arg prefix "$DNS_REMARK_PREFIX" '
        .inbounds[]?
        | ( .remark // "" ) as $r
        | select($r | startswith($prefix))
        | [($r | ltrimstr($prefix)), (.settings.address // ""), (.port // empty | tostring)]
        | @tsv
    ' "$XRAY_INBOUND_FILE" 2>/dev/null
}

xray_require_cmd jq
xray_require_cmd uci
xray_require_cmd sed
xray_require_cmd cmp
xray_require_cmd sort
xray_require_cmd grep

if [ ! -f "$XRAY_INBOUND_FILE" ]; then
    xray_die "XRAY inbound file $XRAY_INBOUND_FILE not found"
fi

usage() {
    cat <<'USAGE'
Usage: dns_forward_remove.sh [--list] [DOMAIN_MASK | --all]

Removes dokodemo-door DNS forwardings created by dns_forward_add.sh.

  --list         Show currently configured masks and exit.
  DOMAIN_MASK    Remove only the specified mask.
  --all          Remove every managed mask (default when no mask passed).
USAGE
}

MODE="all"
TARGET_MASK=""
LIST_ONLY=0

while [ "$#" -gt 0 ]; do
    case "$1" in
        -h|--help)
            usage
            exit 0
            ;;
        --list)
            LIST_ONLY=1
            shift
            ;;
        --all)
            MODE="all"
            TARGET_MASK=""
            shift
            ;;
        --*)
            xray_die "Unknown option: $1"
            ;;
        *)
            if [ -n "$TARGET_MASK" ]; then
                xray_die "Only one DOMAIN_MASK may be specified"
            fi
            MODE="single"
            TARGET_MASK="$1"
            shift
            ;;
    esac
done

forwardings=$(collect_forwardings || true)

if [ "$LIST_ONLY" -eq 1 ]; then
    if [ -z "$forwardings" ]; then
        xray_log "No DNS forwardings configured"
    else
        printf '%s\n' "$forwardings" | while IFS="\t" read -r mask addr port; do
            if [ -n "$port" ]; then
                printf '%s -> %s (port %s)\n' "$mask" "${addr:-?}" "$port"
            else
                printf '%s -> %s\n' "$mask" "${addr:-?}"
            fi
        done
    fi
    exit 0
fi

if [ "$MODE" = "single" ]; then
    TARGET_MASK=$(trim "$TARGET_MASK")
    if ! validate_domain_mask "$TARGET_MASK"; then
        xray_die "Domain mask must look like '*.corp.test.com' or 'corp.test.com'"
    fi
fi

if [ -z "$forwardings" ]; then
    xray_log "No DNS forwardings configured"
    exit 0
fi

selected_masks=""

if [ "$MODE" = "single" ]; then
    if printf '%s\n' "$forwardings" | cut -f1 | grep -Fx "$TARGET_MASK" >/dev/null 2>&1; then
        selected_masks="$TARGET_MASK"
    else
        xray_die "No forwarding found for mask $TARGET_MASK"
    fi
else
    selected_masks=$(printf '%s\n' "$forwardings" | cut -f1 | sort -u)
fi

remarks_json=$(printf '%s\n' "$selected_masks" | jq -Rn --arg prefix "$DNS_REMARK_PREFIX" '[inputs | select(length > 0) | $prefix + .]')

if [ "$remarks_json" = "[]" ]; then
    xray_log "Nothing to remove"
    exit 0
fi

tmp_inbound=$(mktemp)
trap 'rm -f "$tmp_inbound"' EXIT

if ! jq --argjson remarks "$remarks_json" '
    if (.inbounds | type) != "array" then
        error("inbounds must be an array")
    else
        .inbounds = [
            .inbounds[]?
            | ( .remark // "" ) as $r
            | select(( $remarks | index($r) ) | not)
        ]
    end
' "$XRAY_INBOUND_FILE" >"$tmp_inbound"; then
    xray_die "Failed to update $XRAY_INBOUND_FILE"
fi

inbound_changed=0
if ! cmp -s "$tmp_inbound" "$XRAY_INBOUND_FILE"; then
    chmod 0644 "$tmp_inbound"
    mv "$tmp_inbound" "$XRAY_INBOUND_FILE"
    trap - EXIT
    inbound_changed=1
    xray_log "Updated $XRAY_INBOUND_FILE"
else
    rm -f "$tmp_inbound"
    trap - EXIT
    xray_log "No changes required in $XRAY_INBOUND_FILE"
fi

remaining_forwardings=$(collect_forwardings || true)
remaining_base_domains=""
if [ -n "$remaining_forwardings" ]; then
    remaining_base_domains=$(printf '%s\n' "$remaining_forwardings" | cut -f1 | while IFS= read -r mask; do
        base=$(derive_base_domain "$mask")
        printf '%s\n' "$base"
    done | sort -u)
fi

uci_changed=0

servers_output=$(uci -q show "$DNSMASQ_SECTION.server" 2>/dev/null || true)
if [ -n "$servers_output" ]; then
    while IFS= read -r line; do
        value=${line#*=}
        value=${value%\'}
        value=${value#\'}
        value=${value%\"}
        value=${value#\"}
        if [ -z "$value" ]; then
            continue
        fi
        printf '%s\n' "$selected_masks" | while IFS= read -r mask; do
            case "$value" in
                "/$mask/"*)
                    uci del_list "$DNSMASQ_SECTION.server=$value"
                    uci_changed=1
                    break
                    ;;
            esac
        done
    done <<EOF
$servers_output
EOF
fi

rebind_output=$(uci -q show "$DNSMASQ_SECTION.rebind_domain" 2>/dev/null || true)
if [ -n "$rebind_output" ]; then
    while IFS= read -r line; do
        value=${line#*=}
        value=${value%\'}
        value=${value#\'}
        value=${value%\"}
        value=${value#\"}
        if [ -z "$value" ]; then
            continue
        fi
        for mask in $selected_masks; do
            base=$(derive_base_domain "$mask")
            if [ "$value" = "$base" ]; then
                if ! printf '%s\n' "$remaining_base_domains" | grep -Fx "$value" >/dev/null 2>&1; then
                    uci del_list "$DNSMASQ_SECTION.rebind_domain=$value"
                    uci_changed=1
                fi
                break
            fi
        done
    done <<EOF
$rebind_output
EOF
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

xray_log "Removed masks: $(printf '%s' "$selected_masks" | tr '\n' ' ')"
