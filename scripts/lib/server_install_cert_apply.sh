#!/bin/sh
# shellcheck shell=ash

[ "${SERVER_INSTALL_CERT_APPLY_LIB_LOADED:-0}" = "1" ] && return 0
SERVER_INSTALL_CERT_APPLY_LIB_LOADED=1

XRAYP2P_DEFAULT_INBOUNDS="/etc/xray-p2p/inbounds.json"

# When executed directly ensure XRAY_SELF_DIR points to script location
if [ -z "${XRAY_SELF_DIR:-}" ]; then
    case "$0" in
        */*)
            XRAY_SELF_DIR=$(CDPATH= cd -- "$(dirname "$0")" 2>/dev/null && pwd)
            export XRAY_SELF_DIR
            ;;
    esac
fi
: "${XRAY_SELF_DIR:=}"

# Ensure common helpers are available when sourced standalone
xray_cert_apply_try_load_common() {
    command -v xray_common_try_source >/dev/null 2>&1 && return 0

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

    command -v xray_common_try_source >/dev/null 2>&1 || return 1
    load_common_lib >/dev/null 2>&1 || true
}

xray_cert_apply_log() {
    if command -v xray_log >/dev/null 2>&1; then
        xray_log "$@"
    else
        printf '%s\n' "$*" >&2
    fi
}

xray_cert_apply_warn() {
    if command -v xray_warn >/dev/null 2>&1; then
        xray_warn "$@"
    else
        printf 'Warning: %s\n' "$*" >&2
    fi
}

server_install_cert_apply_require_helpers() {
    xray_cert_apply_try_load_common >/dev/null 2>&1 || true
    if ! command -v xray_common_try_source >/dev/null 2>&1; then
        xray_cert_apply_warn "Unable to load XRAY common helpers; continuing without advanced logging."
    fi

    if [ -n "${XRAY_SERVER_CERT_PATHS_LIB:-}" ] && [ -r "${XRAY_SERVER_CERT_PATHS_LIB}" ]; then
        # shellcheck disable=SC1090
        . "${XRAY_SERVER_CERT_PATHS_LIB}"
        if command -v server_cert_paths_update >/dev/null 2>&1; then
            return 0
        fi
    fi

    if command -v server_cert_paths_update >/dev/null 2>&1; then
        return 0
    fi

    if command -v xray_common_try_source >/dev/null 2>&1; then
        if xray_common_try_source \
            "${XRAY_SERVER_CERT_PATHS_LIB:-scripts/lib/server_cert_paths.sh}" \
            "scripts/lib/server_cert_paths.sh" \
            "lib/server_cert_paths.sh"; then
            return 0
        fi
    fi

    for candidate in \
        "scripts/lib/server_cert_paths.sh" \
        "lib/server_cert_paths.sh"; do
        if [ -r "$candidate" ]; then
            # shellcheck disable=SC1090
            . "$candidate"
            return 0
        fi
    done

    xray_cert_apply_warn "Unable to locate server_cert_paths helper."
    return 1
}

server_install_cert_apply_validate() {
    cert="$1"
    key="$2"

    if [ ! -r "$cert" ]; then
        xray_cert_apply_warn "Certificate file is not readable: $cert"
        return 1
    fi
    if [ ! -r "$key" ]; then
        xray_cert_apply_warn "Key file is not readable: $key"
        return 1
    fi

    if ! command -v openssl >/dev/null 2>&1; then
        xray_cert_apply_warn "OpenSSL is not available; skipping certificate validation."
        return 0
    fi

    if ! openssl x509 -in "$cert" -noout >/dev/null 2>&1; then
        xray_cert_apply_warn "Provided certificate is not a valid X.509 file: $cert"
        return 1
    fi
    key_ok=0
    if openssl pkey -in "$key" -noout >/dev/null 2>&1; then
        key_ok=1
    elif openssl rsa -in "$key" -check -noout >/dev/null 2>&1; then
        key_ok=1
    elif openssl ec -in "$key" -check -noout >/dev/null 2>&1; then
        key_ok=1
    fi
    if [ "$key_ok" -ne 1 ]; then
        xray_cert_apply_warn "Provided key is not a recognised private key format: $key"
        return 1
    fi

    cert_hash=$(openssl x509 -in "$cert" -noout -pubkey 2>/dev/null | openssl dgst -sha256 2>/dev/null | awk '{print $NF}')
    key_hash=$(openssl pkey -in "$key" -pubout 2>/dev/null | openssl dgst -sha256 2>/dev/null | awk '{print $NF}')
    if [ -n "$cert_hash" ] && [ -n "$key_hash" ] && [ "$cert_hash" != "$key_hash" ]; then
        xray_cert_apply_warn "Certificate and key do not match (public key mismatch)."
        return 1
    fi

    return 0
}

server_install_cert_apply_paths() {
    inbounds_file="$1"
    cert="$2"
    key="$3"

    [ -n "$inbounds_file" ] || inbounds_file="$XRAYP2P_DEFAULT_INBOUNDS"

    if [ -z "$cert" ] || [ -z "$key" ]; then
        xray_cert_apply_warn "Both certificate and key paths are required."
        return 1
    fi

    if [ ! -f "$inbounds_file" ]; then
        xray_cert_apply_warn "Inbound configuration file not found: $inbounds_file"
        return 1
    fi

    if ! server_install_cert_apply_validate "$cert" "$key"; then
        return 1
    fi

    if ! server_install_cert_apply_require_helpers; then
        return 1
    fi

    if ! server_cert_paths_update "$inbounds_file" "$cert" "$key"; then
        return 1
    fi

    xray_cert_apply_log "Applied certificate ($cert) and key ($key) to $inbounds_file."
    return 0
}

server_install_cert_apply_usage() {
    cat <<EOF
Usage: server_install_cert_apply.sh --cert CERT_FILE --key KEY_FILE [--inbounds FILE]

Apply existing TLS certificate and key paths to the XRAY trojan inbound.
Designed to complement scripts/server.sh --cert/--key flow.

Options:
  --cert CERT_FILE     Path to the certificate (PEM).
  --key KEY_FILE       Path to the private key.
  --inbounds FILE      Path to inbounds.json (default: /etc/xray-p2p/inbounds.json).
  -h, --help           Show this help message.
EOF
}

server_install_cert_apply_main() {
    inbounds="$XRAYP2P_DEFAULT_INBOUNDS"
    cert=""
    key=""

    while [ "$#" -gt 0 ]; do
        case "$1" in
            --cert)
                shift
                cert="$1"
                ;;
            --key)
                shift
                key="$1"
                ;;
            --inbounds)
                shift
                inbounds="$1"
                ;;
            -h|--help)
                server_install_cert_apply_usage
                exit 0
                ;;
            --)
                shift
                break
                ;;
            -*)
                xray_cert_apply_warn "Unknown option: $1"
                server_install_cert_apply_usage
                exit 1
                ;;
            *)
                xray_cert_apply_warn "Unexpected argument: $1"
                server_install_cert_apply_usage
                exit 1
                ;;
        esac
        shift
    done

    if [ -z "$cert" ] || [ -z "$key" ]; then
        xray_cert_apply_warn "Both --cert and --key are required."
        server_install_cert_apply_usage
        exit 1
    fi

    if ! server_install_cert_apply_paths "$inbounds" "$cert" "$key"; then
        exit 1
    fi
}

case "${0##*/}" in
    server_install_cert_apply.sh)
        server_install_cert_apply_main "$@"
        ;;
esac
