#!/bin/sh
set -eu

umask 077

log() {
    printf '%s\n' "$*" >&2
}

die() {
    printf 'Error: %s\n' "$*" >&2
    exit 1
}

require_cmd() {
    cmd="$1"
    if ! command -v "$cmd" >/dev/null 2>&1; then
        case "$cmd" in
            jq)
                die "Required command 'jq' not found. Install it before running this script. For OpenWrt run: opkg update && opkg install jq"
                ;;
            *)
                die "Required command '$cmd' not found. Install it before running this script."
                ;;
        esac
    fi
}

check_repo_access() {
    [ "${XRAY_SKIP_REPO_CHECK:-0}" = "1" ] && return

    base_url="${XRAY_REPO_BASE_URL:-https://raw.githubusercontent.com/NlightN22/xray-p2p/main}"
    check_path="${XRAY_REPO_CHECK_PATH:-scripts/list_clients.sh}"
    timeout="${XRAY_REPO_CHECK_TIMEOUT:-5}"

    base_trimmed="${base_url%/}"
    case "$check_path" in
        /*)
            repo_url="${base_trimmed}${check_path}"
            ;;
        *)
            repo_url="${base_trimmed}/${check_path}"
            ;;
    esac

    last_tool=""
    for tool in curl wget; do
        case "$tool" in
            curl)
                command -v curl >/dev/null 2>&1 || continue
                if curl -fsSL --max-time "$timeout" "$repo_url" >/dev/null 2>&1; then
                    return
                fi
                last_tool="curl"
                ;;
            wget)
                command -v wget >/dev/null 2>&1 || continue
                if wget -q -T "$timeout" -O /dev/null "$repo_url"; then
                    return
                fi
                last_tool="wget"
                ;;
        esac
    done

    if [ -z "$last_tool" ]; then
        log "Neither curl nor wget is available to verify repository accessibility; skipping check."
        return
    fi

    die "Unable to access repository resource $repo_url (last attempt via $last_tool). Set XRAY_SKIP_REPO_CHECK=1 to bypass."
}

CONFIG_DIR="${XRAY_CONFIG_DIR:-/etc/xray}"
INBOUNDS_FILE="${XRAY_INBOUNDS_FILE:-$CONFIG_DIR/inbounds.json}"
CLIENTS_DIR="${XRAY_CLIENTS_DIR:-$CONFIG_DIR/config}"
CLIENTS_FILE="${XRAY_CLIENTS_FILE:-$CLIENTS_DIR/clients.json}"

require_cmd jq

check_repo_access

if [ -f "$CLIENTS_FILE" ]; then
    if ! jq empty "$CLIENTS_FILE" >/dev/null 2>&1; then
        die "Clients registry $CLIENTS_FILE contains invalid JSON."
    fi
fi

if [ -f "$INBOUNDS_FILE" ]; then
    if ! jq empty "$INBOUNDS_FILE" >/dev/null 2>&1; then
        die "Inbound configuration $INBOUNDS_FILE contains invalid JSON."
    fi
fi

print_clients_registry() {
    if [ ! -f "$CLIENTS_FILE" ]; then
        log "Clients registry file not found: $CLIENTS_FILE"
        return 1
    fi

    total_clients=$(jq 'length' "$CLIENTS_FILE")
    printf 'Clients registry (%s): %s entries\n' "$CLIENTS_FILE" "$total_clients"

    if [ "$total_clients" -eq 0 ]; then
        printf '  (no clients)\n'
        return 0
    fi

    jq -r '
        sort_by(.email // "") |
        map(
            "- "
            + (.email // "<unknown>")
            + " | status=" + (.status // "unknown")
            + (if (.id // "") != "" then " | id=" + (.id // "") else "" end)
            + (if (.issued_at // "") != "" then " | issued=" + (.issued_at // "") else "" end)
            + (if (.activated_at // "") != "" then " | activated=" + (.activated_at // "") else "" end)
            + (if (.issued_by // "") != "" then " | issued_by=" + (.issued_by // "") else "" end)
            + (if (.notes // "") != "" then " | notes=" + ((.notes // "") | gsub("\\s+"; " ")) else "" end)
        )[]
    ' "$CLIENTS_FILE"

    if jq -e 'map(select((.link // "") != "")) | length > 0' "$CLIENTS_FILE" >/dev/null 2>&1; then
        printf '\n'
        printf 'Links available for clients with non-empty link field. Use jq to extract full URLs if needed.\n'
    fi
}

print_trojan_inbounds() {
    if [ ! -f "$INBOUNDS_FILE" ]; then
        log "Inbound configuration file not found: $INBOUNDS_FILE"
        return 1
    fi

    trojan_output=$(jq -r '
        ( .inbounds // [] )
        | map(select((.protocol // "") == "trojan")
            | ( .streamSettings // {} ) as $stream
            | ( $stream.tlsSettings // {} ) as $tls
            | {
                port: (if (.port? == null) then "unknown" else (.port | tostring) end),
                sni: ($tls.serverName // ""),
                clients: (.settings.clients // [])
            }
        )
        | if length == 0 then empty else
            map(
                "Port " + .port
                + (if (.sni // "") != "" then " (sni=" + .sni + ")" else "" end)
                + ":\n"
                + (if (.clients | length) == 0 then "  (no clients)"
                   else (.clients
                        | map("  - " + (.email // "<unknown>") + " | password=" + (.password // "<missing>"))
                        | join("\n"))
                   end)
            )
            | join("\n")
        end
    ' "$INBOUNDS_FILE")

    if [ -z "$trojan_output" ]; then
        printf 'No trojan inbounds found in %s.\n' "$INBOUNDS_FILE"
    else
        printf 'Trojan inbound clients (%s):\n' "$INBOUNDS_FILE"
        printf '%s\n' "$trojan_output"
    fi
}

print_mismatches() {
    [ ! -f "$CLIENTS_FILE" ] && return 0
    [ ! -f "$INBOUNDS_FILE" ] && return 0

    mismatch_json=$(jq -n \
        --argfile registry "$CLIENTS_FILE" \
        --argfile inbound "$INBOUNDS_FILE" '
            {
                registry: ($registry // [] | map(.email // "") | map(select(length > 0)) | unique),
                inbound: ($inbound.inbounds // []
                    | map(select((.protocol // "") == "trojan") | (.settings.clients // []))
                    | add // []
                    | map(.email // "")
                    | map(select(length > 0))
                    | unique)
            }
            | {
                only_registry: (.registry - .inbound),
                only_inbound: (.inbound - .registry)
            }
        ')

    only_registry=$(printf '%s' "$mismatch_json" | jq -r '.only_registry[]?' 2>/dev/null || true)
    only_inbound=$(printf '%s' "$mismatch_json" | jq -r '.only_inbound[]?' 2>/dev/null || true)

    if [ -n "$only_registry" ] || [ -n "$only_inbound" ]; then
        printf '\nMismatches between registry and inbounds:\n'
        if [ -n "$only_registry" ]; then
            printf '  Present only in %s:\n' "$CLIENTS_FILE"
            printf '%s\n' "$only_registry" | sed 's/^/    - /'
        fi
        if [ -n "$only_inbound" ]; then
            printf '  Present only in trojan inbound configuration:\n'
            printf '%s\n' "$only_inbound" | sed 's/^/    - /'
        fi
    fi
}

print_clients_registry
printf '\n'
print_trojan_inbounds
print_mismatches
