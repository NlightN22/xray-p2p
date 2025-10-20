#!/bin/sh
# shellcheck shell=ash

[ "${SERVER_CERT_PATHS_LIB_LOADED:-0}" = "1" ] && return 0
SERVER_CERT_PATHS_LIB_LOADED=1

# Determine XRAY_SELF_DIR when invoked directly
if [ -z "${XRAY_SELF_DIR:-}" ]; then
    case "$0" in
        */*)
            XRAY_SELF_DIR=$(CDPATH= cd -- "$(dirname "$0")" 2>/dev/null && pwd)
            export XRAY_SELF_DIR
            ;;
    esac
fi
: "${XRAY_SELF_DIR:=}"

server_cert_paths_bootstrap_common() {
    if command -v xray_log >/dev/null 2>&1 && command -v xray_warn >/dev/null 2>&1; then
        return 0
    fi

    if ! command -v xray_common_try_source >/dev/null 2>&1; then
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

        if ! command -v load_common_lib >/dev/null 2>&1; then
            base="${XRAY_REPO_BASE_URL:-https://raw.githubusercontent.com/NlightN22/xray-p2p/main}"
            loader_url="${base%/}/scripts/lib/common_loader.sh"
            tmp="$(mktemp 2>/dev/null)" || return 1
            if command -v curl >/dev/null 2>&1 && curl -fsSL "$loader_url" -o "$tmp"; then
                :
            elif command -v wget >/dev/null 2>&1 && wget -q -O "$tmp" "$loader_url"; then
                :
            else
                rm -f "$tmp"
                return 1
            fi
            # shellcheck disable=SC1090
            . "$tmp"
            rm -f "$tmp"
        fi
    fi

    if command -v load_common_lib >/dev/null 2>&1; then
        load_common_lib >/dev/null 2>&1 || true
    fi

    if ! command -v xray_log >/dev/null 2>&1 || ! command -v xray_warn >/dev/null 2>&1; then
        for candidate in \
            "${XRAY_SELF_DIR%/}/scripts/lib/common.sh" \
            "scripts/lib/common.sh" \
            "lib/common.sh"; do
            if [ -n "$candidate" ] && [ -r "$candidate" ]; then
                # shellcheck disable=SC1090
                . "$candidate"
                break
            fi
        done
    fi
}

server_cert_paths_bootstrap_common

server_cert_paths_usage() {
    cat <<EOF
Usage: server_cert_paths.sh [--inbounds FILE] [--index N | --port P] CERT_FILE KEY_FILE

Assign certificate and key file paths into trojan inbound(s) in inbounds.json.
Validates files and warns on issues; always writes paths if possible.

Options:
  --inbounds FILE   Path to inbounds.json (default: /etc/xray-p2p/inbounds.json or XRAY_INBOUNDS_FILE).
  --index N         Select N-th trojan inbound (1-based) when multiple exist.
  --port P          Select trojan inbound by port number.
  -h, --help        Show this help message.

Environment:
  XRAY_INBOUNDS_FILE   Overrides default inbounds.json path.
  XRAY_TROJAN_INDEX    1-based index to select trojan inbound (used if not interactive).
  XRAY_TROJAN_PORT     Port to select trojan inbound (used if not interactive).
EOF
}

server_cert_paths_require_jq() {
    if command -v jq >/dev/null 2>&1; then
        return 0
    fi
    if command -v xray_die >/dev/null 2>&1; then
        xray_die "Required command 'jq' not found. Install it first."
    else
        printf 'Error: Required command jq not found.\n' >&2
        exit 1
    fi
}

server_cert_paths_log() {
    if command -v xray_log >/dev/null 2>&1; then
        xray_log "$@"
    else
        printf '%s\n' "$*" >&2
    fi
}

server_cert_paths_warn() {
    if command -v xray_warn >/dev/null 2>&1; then
        xray_warn "$@"
    else
        printf 'Warning: %s\n' "$*" >&2
    fi
}

server_cert_paths_detect_pubkey_hash() {
    # stdin: PEM pubkey; stdout: hash or empty
    if command -v openssl >/dev/null 2>&1; then
        openssl dgst -sha256 2>/dev/null | awk '{print $NF}' 2>/dev/null
        return 0
    fi
    return 1
}

server_cert_paths_check_material() {
    cert="$1"
    key="$2"

    [ -r "$cert" ] || server_cert_paths_warn "Certificate file not readable: $cert"
    [ -r "$key" ] || server_cert_paths_warn "Key file not readable: $key"

    if ! command -v openssl >/dev/null 2>&1; then
        server_cert_paths_warn "OpenSSL not available; skipping certificate/key validation"
        return 0
    fi

    if ! openssl x509 -in "$cert" -noout >/dev/null 2>&1; then
        server_cert_paths_warn "Provided certificate is not a valid X.509: $cert"
    else
        if ! openssl x509 -in "$cert" -checkend 0 -noout >/dev/null 2>&1; then
            server_cert_paths_warn "Certificate appears expired: $cert"
        fi
    fi

    key_ok=0
    if openssl pkey -in "$key" -noout >/dev/null 2>&1; then
        key_ok=1
    elif openssl rsa -in "$key" -check -noout >/dev/null 2>&1; then
        key_ok=1
    elif openssl ec -in "$key" -check -noout >/dev/null 2>&1; then
        key_ok=1
    fi
    [ "$key_ok" -eq 1 ] || server_cert_paths_warn "Key file does not look valid: $key"

    cert_hash=""
    key_hash=""
    cert_hash=$(openssl x509 -in "$cert" -noout -pubkey 2>/dev/null | server_cert_paths_detect_pubkey_hash 2>/dev/null)
    if [ -n "$cert_hash" ]; then
        if openssl pkey -in "$key" -pubout 2>/dev/null | server_cert_paths_detect_pubkey_hash >/dev/null 2>&1; then
            key_hash=$(openssl pkey -in "$key" -pubout 2>/dev/null | server_cert_paths_detect_pubkey_hash 2>/dev/null)
        elif openssl rsa -in "$key" -pubout 2>/dev/null | server_cert_paths_detect_pubkey_hash >/dev/null 2>&1; then
            key_hash=$(openssl rsa -in "$key" -pubout 2>/dev/null | server_cert_paths_detect_pubkey_hash 2>/dev/null)
        elif openssl ec -in "$key" -pubout 2>/dev/null | server_cert_paths_detect_pubkey_hash >/dev/null 2>&1; then
            key_hash=$(openssl ec -in "$key" -pubout 2>/dev/null | server_cert_paths_detect_pubkey_hash 2>/dev/null)
        fi
    fi

    if [ -n "$cert_hash" ] && [ -n "$key_hash" ] && [ "$cert_hash" != "$key_hash" ]; then
        server_cert_paths_warn "Certificate and key do not match (pubkey hash mismatch)"
    fi
}

server_cert_paths_list() {
    file="$1"
    jq -r '
      [ .inbounds[]? | select((.protocol // "") == "trojan")
        | { tag: (.tag // ""), listen: (.listen // ""), port: (.port // 0) } ]
      | to_entries[]
      | "\(.key+1)|\(.value.tag)|\(.value.listen)|\(.value.port)"' "$file" 2>/dev/null
}

server_cert_paths_select() {
    file="$1"
    sel_index="$2"
    sel_port="$3"

    list=$(server_cert_paths_list "$file")
    [ -n "$list" ] || return 1

    count=$(printf '%s\n' "$list" | wc -l | awk '{print $1}')
    if [ "$count" -eq 1 ]; then
        printf '%s\n' "$list"
        return 0
    fi

    if [ -n "$sel_port" ] && printf '%s\n' "$list" | awk -F'|' -v p="$sel_port" '$4==p' | grep -q .; then
        printf '%s\n' "$list" | awk -F'|' -v p="$sel_port" '$4==p {print; exit 0}'
        return 0
    fi

    if [ -n "$sel_index" ] && printf '%s\n' "$list" | awk -F'|' -v i="$sel_index" '$1==i' | grep -q .; then
        printf '%s\n' "$list" | awk -F'|' -v i="$sel_index" '$1==i {print; exit 0}'
        return 0
    fi

    if [ -t 0 ] || [ -r /dev/tty ]; then
        server_cert_paths_log "Multiple trojan inbounds detected:"
        printf '%s\n' "$list" | awk -F'|' '{printf "  [%s] tag=%s listen=%s port=%s\n", $1, $2, $3, $4}' >&2
        printf 'Select inbound [1-%s]: ' "$count" >&2
        if [ -t 0 ]; then
            IFS= read -r answer
        else
            IFS= read -r answer </dev/tty
        fi
        [ -n "$answer" ] || answer=1
        printf '%s\n' "$list" | awk -F'|' -v i="$answer" '$1==i {print; found=1} END {exit found?0:1}'
        return $?
    fi

    server_cert_paths_warn "Multiple trojan inbounds found; non-interactive mode selects the first"
    printf '%s\n' "$list" | head -n1
}

server_cert_paths_update() {
    inbounds_file="$1"
    cert="$2"
    key="$3"

    [ -n "$inbounds_file" ] || inbounds_file="${XRAY_INBOUNDS_FILE:-/etc/xray-p2p/inbounds.json}"
    [ -f "$inbounds_file" ] || {
        server_cert_paths_warn "Inbound file not found: $inbounds_file"
        return 1
    }

    server_cert_paths_require_jq
    server_cert_paths_check_material "$cert" "$key"

    select_idx="${XRAY_TROJAN_INDEX:-}"
    select_port="${XRAY_TROJAN_PORT:-}"
    selected=$(server_cert_paths_select "$inbounds_file" "$select_idx" "$select_port") || {
        server_cert_paths_warn "No trojan inbounds found in $inbounds_file"
        return 1
    }

    sel_tag=$(printf '%s' "$selected" | awk -F'|' '{print $2}')
    sel_listen=$(printf '%s' "$selected" | awk -F'|' '{print $3}')
    sel_port=$(printf '%s' "$selected" | awk -F'|' '{print $4}')

    tmp=$(mktemp 2>/dev/null) || tmp="$inbounds_file.tmp"
    if ! jq \
        --arg cert "$cert" \
        --arg key "$key" \
        --arg sel_tag "$sel_tag" \
        --arg sel_listen "$sel_listen" \
        --argjson sel_port "$sel_port" '
        .inbounds |= (map(
          if (.protocol // "") == "trojan"
             and ((.tag // "") == $sel_tag)
             and ((.listen // "") == $sel_listen)
             and ((.port // 0) == $sel_port) then
            (.streamSettings //= {})
            | (.streamSettings.security = "tls")
            | (.streamSettings.tlsSettings //= {})
            | (.streamSettings.tlsSettings.certificates //= [ {} ])
            | (.streamSettings.tlsSettings.certificates[0].certificateFile = $cert)
            | (.streamSettings.tlsSettings.certificates[0].keyFile = $key)
          else . end
        ))' "$inbounds_file" >"$tmp" 2>/dev/null; then
        rm -f "$tmp" 2>/dev/null || true
        server_cert_paths_warn "Failed to update $inbounds_file"
        return 1
    fi

    if ! mv "$tmp" "$inbounds_file" 2>/dev/null; then
        cat "$tmp" >"$inbounds_file" 2>/dev/null || {
            rm -f "$tmp" 2>/dev/null || true
            server_cert_paths_warn "Unable to write changes to $inbounds_file"
            return 1
        }
        rm -f "$tmp" 2>/dev/null || true
    fi

    server_cert_paths_log "Updated trojan inbound (tag=${sel_tag:-""} listen=${sel_listen:-""} port=${sel_port:-""}) with certificate and key paths."
    return 0
}

server_cert_paths_main() {
    inbounds_file="${XRAY_INBOUNDS_FILE:-/etc/xray-p2p/inbounds.json}"
    sel_index=""
    sel_port=""

    while [ "$#" -gt 0 ]; do
        case "$1" in
            -h|--help)
                server_cert_paths_usage
                exit 0
                ;;
            --inbounds)
                shift
                inbounds_file="$1"
                ;;
            --index)
                shift
                sel_index="$1"
                ;;
            --port)
                shift
                sel_port="$1"
                ;;
            --)
                shift
                break
                ;;
            -*)
                server_cert_paths_warn "Unknown option: $1"
                server_cert_paths_usage
                exit 1
                ;;
            *)
                break
                ;;
        esac
        shift
    done

    if [ "$#" -lt 2 ]; then
        server_cert_paths_usage
        exit 1
    fi

    cert="$1"
    key="$2"

    XRAY_TROJAN_INDEX="${XRAY_TROJAN_INDEX:-$sel_index}"
    XRAY_TROJAN_PORT="${XRAY_TROJAN_PORT:-$sel_port}"
    export XRAY_TROJAN_INDEX XRAY_TROJAN_PORT

    if ! server_cert_paths_update "$inbounds_file" "$cert" "$key"; then
        exit 1
    fi
}

# If executed directly, run the CLI entrypoint
case "${0##*/}" in
    server_cert_paths.sh)
        server_cert_paths_main "$@"
        ;;
esac
