#!/bin/sh
# shellcheck shell=ash

[ "${SERVER_INSTALL_CERT_LIB_LOADED:-0}" = "1" ] && return 0
SERVER_INSTALL_CERT_LIB_LOADED=1

server_install_cert_file=""
server_install_key_file=""
server_install_cert_exists=0
server_install_key_exists=0

server_install_prepare_cert_paths() {
    inbound_path="$1"
    server_install_cert_file=$(jq -r 'first(.inbounds[]? | .streamSettings? | .tlsSettings? | .certificates[]? | .certificateFile) // empty' "$inbound_path" 2>/dev/null)
    server_install_key_file=$(jq -r 'first(.inbounds[]? | .streamSettings? | .tlsSettings? | .certificates[]? | .keyFile) // empty' "$inbound_path" 2>/dev/null)
    [ -n "$server_install_cert_file" ] || server_install_cert_file="$XRAYP2P_CONFIG_DIR/cert.pem"
    [ -n "$server_install_key_file" ] || server_install_key_file="$XRAYP2P_CONFIG_DIR/key.pem"
    server_install_cert_exists=0
    server_install_key_exists=0
    [ -f "$server_install_cert_file" ] && server_install_cert_exists=1
    [ -f "$server_install_key_file" ] && server_install_key_exists=1
}

server_install_require_openssl() {
    need_openssl=0
    case "${XRAY_REISSUE_CERT:-}" in
        1) need_openssl=1 ;;
        0)
            if [ "$server_install_cert_exists" -eq 0 ] || [ "$server_install_key_exists" -eq 0 ]; then
                need_openssl=1
            fi
            ;;
        *)
            if [ "$server_install_cert_exists" -eq 0 ] || [ "$server_install_key_exists" -eq 0 ]; then
                need_openssl=1
            fi
            ;;
    esac
    [ "$need_openssl" -eq 0 ] || xray_require_cmd openssl
}

server_install_decide_reissue() {
    case "${XRAY_REISSUE_CERT:-}" in
        1)
            xray_log "Regenerating certificate and key (forced by XRAY_REISSUE_CERT=1)"
            server_install_cert_exists=0
            server_install_key_exists=0
            ;;
        0)
            if [ "$server_install_cert_exists" -eq 0 ] || [ "$server_install_key_exists" -eq 0 ]; then
                xray_log "Certificate files are missing; generating new ones despite XRAY_REISSUE_CERT=0."
            else
                xray_log "Keeping existing certificate and key (XRAY_REISSUE_CERT=0)"
            fi
            ;;
        *)
            if [ "$server_install_cert_exists" -eq 1 ] && [ "$server_install_key_exists" -eq 1 ]; then
                xray_log "Existing certificate and key detected."
                if [ -t 0 ]; then
                    if xray_prompt_yes_no "Regenerate TLS certificate and key? [y/N]: " "N"; then
                        server_install_cert_exists=0
                        server_install_key_exists=0
                    fi
                else
                    xray_log "No interactive terminal available. Set XRAY_REISSUE_CERT=1 to regenerate or 0 to keep existing material."
                fi
            fi
            ;;
    esac
}

server_install_read_server_name() {
    if [ -n "$XRAY_SERVER_NAME" ]; then
        XRAY_CERT_NAME="$XRAY_SERVER_NAME"
        xray_log "Using XRAY_SERVER_NAME=$XRAY_CERT_NAME from environment"
        return
    fi

    existing_cn=""
    if [ "$server_install_cert_exists" -eq 1 ] && command -v openssl >/dev/null 2>&1; then
        existing_cn=$(openssl x509 -noout -subject -nameopt RFC2253 -in "$server_install_cert_file" 2>/dev/null | awk -F'CN=' 'NF>1 {print $2}' | cut -d',' -f1 | sed 's/^ *//;s/ *$//')
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
            xray_die "No interactive terminal available. Set XRAY_SERVER_NAME environment variable."
        fi

        [ -n "$XRAY_CERT_NAME" ] || XRAY_CERT_NAME="$existing_cn"
        if [ -z "$XRAY_CERT_NAME" ]; then
            xray_log "Server name cannot be empty."
        elif ! echo "$XRAY_CERT_NAME" | grep -Eq "^[A-Za-z0-9.-]+$"; then
            xray_log "Server name must contain only letters, digits, dots or hyphens."
            XRAY_CERT_NAME=""
        fi
    done
}

server_install_generate_cert() {
    echo "$XRAY_CERT_NAME" | grep -Eq "^[A-Za-z0-9.-]+$" || xray_die "Server name must contain only letters, digits, dots or hyphens."
    cert_dir=$(dirname "$server_install_cert_file")
    key_dir=$(dirname "$server_install_key_file")
    mkdir -p "$cert_dir" "$key_dir"
    suffix=$(date +%Y%m%d%H%M%S)
    if [ -f "$server_install_cert_file" ]; then
        mv "$server_install_cert_file" "${server_install_cert_file}.${suffix}.bak"
    fi
    if [ -f "$server_install_key_file" ]; then
        mv "$server_install_key_file" "${server_install_key_file}.${suffix}.bak"
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
    if ! openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout "$server_install_key_file" -out "$server_install_cert_file" -config "$openssl_cnf" >/dev/null 2>&1; then
        rm -f "$openssl_cnf"
        xray_die "Failed to generate certificate for $XRAY_CERT_NAME"
    fi
    rm -f "$openssl_cnf"
    chmod 600 "$server_install_key_file"
    chmod 644 "$server_install_cert_file"
}

server_install_handle_certificates() {
    inbound_path="$1"
    server_install_prepare_cert_paths "$inbound_path"
    server_install_require_openssl
    server_install_decide_reissue
    if [ "$server_install_cert_exists" -eq 0 ] || [ "$server_install_key_exists" -eq 0 ]; then
        server_install_read_server_name
        server_install_generate_cert
    else
        xray_log "Skipping certificate regeneration; keeping existing files in place."
    fi
}
