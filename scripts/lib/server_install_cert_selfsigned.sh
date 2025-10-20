#!/bin/sh
# shellcheck shell=ash

[ "${SERVER_INSTALL_CERT_SELF_LIB_LOADED:-0}" = "1" ] && return 0
SERVER_INSTALL_CERT_SELF_LIB_LOADED=1

XRAYP2P_DEFAULT_CONFIG_DIR="/etc/xray-p2p"

# Helper logging fallback
server_selfsigned_log() {
    if command -v xray_log >/dev/null 2>&1; then
        xray_log "$@"
    else
        printf '%s\n' "$*" >&2
    fi
}

server_selfsigned_warn() {
    if command -v xray_warn >/dev/null 2>&1; then
        xray_warn "$@"
    else
        printf 'Warning: %s\n' "$*" >&2
    fi
}

server_selfsigned_die() {
    if command -v xray_die >/dev/null 2>&1; then
        xray_die "$@"
    else
        printf 'Error: %s\n' "$*" >&2
        exit 1
    fi
}

server_selfsigned_require_cmd() {
    cmd="$1"
    if command -v xray_require_cmd >/dev/null 2>&1; then
        xray_require_cmd "$cmd"
    else
        command -v "$cmd" >/dev/null 2>&1 || server_selfsigned_die "Required command '$cmd' not found."
    fi
}

server_selfsigned_prompt_yes_no() {
    prompt="$1"
    default="$2"
    if command -v xray_prompt_yes_no >/dev/null 2>&1; then
        xray_prompt_yes_no "$prompt" "$default"
        return $?
    fi

    printf "%s" "$prompt" >&2
    IFS= read -r answer
    [ -n "$answer" ] || answer="$default"
    case "$answer" in
        Y|y|yes|YES) return 0 ;;
        *) return 1 ;;
    esac
}

server_selfsigned_prepare_paths() {
    inbound_path="$1"
    command -v jq >/dev/null 2>&1 || server_selfsigned_die "Required command 'jq' not found."
    config_dir="${XRAYP2P_CONFIG_DIR:-$XRAYP2P_DEFAULT_CONFIG_DIR}"
    server_selfsigned_cert_file=$(jq -r 'first(.inbounds[]? | .streamSettings? | .tlsSettings? | .certificates[]? | .certificateFile) // empty' "$inbound_path" 2>/dev/null)
    server_selfsigned_key_file=$(jq -r 'first(.inbounds[]? | .streamSettings? | .tlsSettings? | .certificates[]? | .keyFile) // empty' "$inbound_path" 2>/dev/null)
    [ -n "$server_selfsigned_cert_file" ] || server_selfsigned_cert_file="$config_dir/cert.pem"
    [ -n "$server_selfsigned_key_file" ] || server_selfsigned_key_file="$config_dir/key.pem"
    server_selfsigned_cert_exists=0
    server_selfsigned_key_exists=0
    [ -f "$server_selfsigned_cert_file" ] && server_selfsigned_cert_exists=1
    [ -f "$server_selfsigned_key_file" ] && server_selfsigned_key_exists=1
}

server_selfsigned_require_openssl() {
    need_openssl=0
    case "${XRAY_REISSUE_CERT:-}" in
        1) need_openssl=1 ;;
        0)
            if [ "$server_selfsigned_cert_exists" -eq 0 ] || [ "$server_selfsigned_key_exists" -eq 0 ]; then
                need_openssl=1
            fi
            ;;
        *)
            if [ "$server_selfsigned_cert_exists" -eq 0 ] || [ "$server_selfsigned_key_exists" -eq 0 ]; then
                need_openssl=1
            fi
            ;;
    esac
    [ "$need_openssl" -eq 0 ] || server_selfsigned_require_cmd openssl
}

server_selfsigned_decide_reissue() {
    case "${XRAY_REISSUE_CERT:-}" in
        1)
            server_selfsigned_log "Regenerating certificate and key (forced by XRAY_REISSUE_CERT=1)"
            server_selfsigned_cert_exists=0
            server_selfsigned_key_exists=0
            ;;
        0)
            if [ "$server_selfsigned_cert_exists" -eq 0 ] || [ "$server_selfsigned_key_exists" -eq 0 ]; then
                server_selfsigned_log "Certificate files are missing; generating new ones despite XRAY_REISSUE_CERT=0."
            else
                server_selfsigned_log "Keeping existing certificate and key (XRAY_REISSUE_CERT=0)"
            fi
            ;;
        *)
            if [ "$server_selfsigned_cert_exists" -eq 1 ] && [ "$server_selfsigned_key_exists" -eq 1 ]; then
                server_selfsigned_log "Existing certificate and key detected."
                if [ -t 0 ]; then
                    if server_selfsigned_prompt_yes_no "Regenerate TLS certificate and key? [y/N]: " "N"; then
                        server_selfsigned_cert_exists=0
                        server_selfsigned_key_exists=0
                    fi
                else
                    server_selfsigned_log "No interactive terminal available. Set XRAY_REISSUE_CERT=1 to regenerate or 0 to keep existing material."
                fi
            fi
            ;;
    esac
}

server_selfsigned_read_name() {
    if [ -n "$XRAY_SERVER_NAME" ]; then
        XRAY_CERT_NAME="$XRAY_SERVER_NAME"
        server_selfsigned_log "Using XRAY_SERVER_NAME=$XRAY_CERT_NAME from environment"
        return
    fi

    existing_cn=""
    if [ "$server_selfsigned_cert_exists" -eq 1 ] && command -v openssl >/dev/null 2>&1; then
        existing_cn=$(openssl x509 -noout -subject -nameopt RFC2253 -in "$server_selfsigned_cert_file" 2>/dev/null | awk -F'CN=' 'NF>1 {print $2}' | cut -d',' -f1 | sed 's/^ *//;s/ *$//')
    fi

    XRAY_CERT_NAME=""
    while [ -z "$XRAY_CERT_NAME" ]; do
        if [ -n "$existing_cn" ]; then
            printf "Enter server name for TLS certificate [%s]: " "$existing_cn" >&2
        else
            printf "Enter server name for TLS certificate (e.g. vpn.example.com): " >&2
        fi

        if [ -t 0 ]; then
            IFS= read -r XRAY_CERT_NAME
        elif [ -r /dev/tty ]; then
            IFS= read -r XRAY_CERT_NAME </dev/tty
        else
            server_selfsigned_die "No interactive terminal available. Set XRAY_SERVER_NAME environment variable."
        fi

        [ -n "$XRAY_CERT_NAME" ] || XRAY_CERT_NAME="$existing_cn"
        if [ -z "$XRAY_CERT_NAME" ]; then
            server_selfsigned_log "Server name cannot be empty."
        elif ! echo "$XRAY_CERT_NAME" | grep -Eq "^[A-Za-z0-9.-]+$"; then
            server_selfsigned_log "Server name must contain only letters, digits, dots or hyphens."
            XRAY_CERT_NAME=""
        fi
    done
}

server_selfsigned_generate() {
    echo "$XRAY_CERT_NAME" | grep -Eq "^[A-Za-z0-9.-]+$" || server_selfsigned_die "Server name must contain only letters, digits, dots or hyphens."
    cert_dir=$(dirname "$server_selfsigned_cert_file")
    key_dir=$(dirname "$server_selfsigned_key_file")
    mkdir -p "$cert_dir" "$key_dir"
    suffix=$(date +%Y%m%d%H%M%S)
    if [ -f "$server_selfsigned_cert_file" ]; then
        mv "$server_selfsigned_cert_file" "${server_selfsigned_cert_file}.${suffix}.bak"
    fi
    if [ -f "$server_selfsigned_key_file" ]; then
        mv "$server_selfsigned_key_file" "${server_selfsigned_key_file}.${suffix}.bak"
    fi

    openssl_cnf=$(mktemp)
    cat >"$openssl_cnf" <<EOF
[req]
prompt = no
default_bits = 2048
default_md = sha256
req_extensions = req_ext
distinguished_name = dn

[dn]
CN = $XRAY_CERT_NAME

[req_ext]
subjectAltName = @alt_names

[alt_names]
DNS.1 = $XRAY_CERT_NAME
EOF
    if ! openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout "$server_selfsigned_key_file" -out "$server_selfsigned_cert_file" -config "$openssl_cnf" >/dev/null 2>&1; then
        rm -f "$openssl_cnf"
        server_selfsigned_die "Failed to generate certificate for $XRAY_CERT_NAME"
    fi
    rm -f "$openssl_cnf"
    chmod 600 "$server_selfsigned_key_file"
    chmod 644 "$server_selfsigned_cert_file"
    server_selfsigned_log "Self-signed certificate generated at cert=$server_selfsigned_cert_file key=$server_selfsigned_key_file"
}

server_install_selfsigned_handle() {
    inbound_path="$1"
    [ -n "$inbound_path" ] || inbound_path="${XRAYP2P_DEFAULT_CONFIG_DIR}/inbounds.json"
    [ -f "$inbound_path" ] || server_selfsigned_die "Inbound configuration does not exist: $inbound_path"

    server_selfsigned_prepare_paths "$inbound_path"
    server_selfsigned_require_openssl
    server_selfsigned_decide_reissue
    if [ "$server_selfsigned_cert_exists" -eq 0 ] || [ "$server_selfsigned_key_exists" -eq 0 ]; then
        server_selfsigned_read_name
        server_selfsigned_generate
    else
        server_selfsigned_log "Skipping certificate regeneration; keeping existing files in place."
    fi
}

server_install_selfsigned_usage() {
    cat <<EOF
Usage: server_install_cert_selfsigned.sh [--inbounds FILE]

Generate or refresh a self-signed certificate for the XRAY trojan inbound.
Respects XRAY_REISSUE_CERT and XRAY_SERVER_NAME environment variables.

Options:
  --inbounds FILE  Path to inbounds.json (default: /etc/xray-p2p/inbounds.json).
  -h, --help       Show this help message.
EOF
}

server_install_selfsigned_main() {
    inbounds="${XRAYP2P_DEFAULT_CONFIG_DIR}/inbounds.json"

    while [ "$#" -gt 0 ]; do
        case "$1" in
            --inbounds)
                shift
                inbounds="$1"
                ;;
            -h|--help)
                server_install_selfsigned_usage
                exit 0
                ;;
            --)
                shift
                break
                ;;
            -*)
                server_selfsigned_log "Unknown option: $1"
                server_install_selfsigned_usage
                exit 1
                ;;
            *)
                server_selfsigned_log "Unexpected argument: $1"
                server_install_selfsigned_usage
                exit 1
                ;;
        esac
        shift
    done

    if ! server_install_selfsigned_handle "$inbounds"; then
        exit 1
    fi
}

case "${0##*/}" in
    server_install_cert_selfsigned.sh)
        server_install_selfsigned_main "$@"
        ;;
esac
