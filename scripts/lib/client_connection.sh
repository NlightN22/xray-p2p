#!/bin/sh

[ "${CLIENT_CONNECTION_LIB_LOADED:-0}" = "1" ] && return 0
CLIENT_CONNECTION_LIB_LOADED=1

client_connection_reset() {
    CLIENT_CONNECTION_URL=""
    CLIENT_CONNECTION_PASSWORD=""
    CLIENT_CONNECTION_HOST=""
    CLIENT_CONNECTION_PORT=""
    CLIENT_CONNECTION_SERVER_NAME=""
    CLIENT_CONNECTION_NETWORK="tcp"
    CLIENT_CONNECTION_SECURITY="tls"
    CLIENT_CONNECTION_ALLOW_INSECURE="true"
    CLIENT_CONNECTION_LABEL=""
    CLIENT_CONNECTION_TAG=""
}

client_connection_reset

client_connection_sanitize_tag() {
    local source="$1"
    local sanitized=""

    sanitized=$(printf '%s' "$source" | tr '[:upper:]' '[:lower:]')
    sanitized=$(printf '%s' "$sanitized" | sed 's/[^0-9a-z._-]/-/g; s/--*/-/g; s/^-*//; s/-*$//')

    printf '%s' "$sanitized"
}

client_connection_generate_tag() {
    local host="$1"
    local port="$2"
    local base

    base=$(client_connection_sanitize_tag "$host")
    [ -n "$base" ] || base="proxy"

    printf '%s-%s' "$base" "$port"
}

client_connection_parse() {
    local url="$1"
    local without_proto=""
    local fragment=""
    local main_part=""
    local query=""
    local base_part=""
    local password_part=""
    local server_part=""
    local host=""
    local port=""
    local remain=""
    local pair=""
    local key=""
    local value=""
    local network_type="tcp"
    local security_type="tls"
    local allow_insecure_value="true"
    local server_name=""
    local port_num=""

    client_connection_reset

    [ -n "$url" ] || xray_die "Connection string is required"

    case "$url" in
        trojan://*) ;;
        *)
            xray_die "Unsupported protocol in connection string. Expected trojan://"
            ;;
    esac

    without_proto="${url#trojan://}"

    fragment=""
    main_part="$without_proto"
    case "$main_part" in
        *'#'*)
            fragment="${main_part#*#}"
            main_part="${main_part%%#*}"
            ;;
    esac

    query=""
    base_part="$main_part"
    case "$main_part" in
        *'?'*)
            query="${main_part#*\?}"
            base_part="${main_part%%\?*}"
            ;;
    esac

    if [ "${base_part#*@}" = "$base_part" ]; then
        xray_die "Connection string is missing password (expected password@host:port)"
    fi

    password_part="${base_part%%@*}"
    server_part="${base_part#*@}"

    if [ -z "$password_part" ]; then
        xray_die "Password part of the connection string is empty"
    fi

    case "$server_part" in
        \[*\]*)
            host="${server_part%%]*}"
            host="${host#[}"
            remain="${server_part#*]}"
            remain="${remain#*:}"
            port="$remain"
            ;;
        *)
            if [ "${server_part##*:}" = "$server_part" ]; then
                xray_die "Connection string is missing port"
            fi
            port="${server_part##*:}"
            host="${server_part%:*}"
            ;;
    esac

    if [ -z "$host" ]; then
        xray_die "Host portion of the connection string is empty"
    fi

    if [ -z "$port" ]; then
        xray_die "Port portion of the connection string is empty"
    fi

    case "$port" in
        ''|*[!0-9]*)
            xray_die "Port must be numeric"
            ;;
    esac

    port_num=$port
    if [ "$port_num" -le 0 ] || [ "$port_num" -gt 65535 ]; then
        xray_die "Port must be between 1 and 65535"
    fi

    server_name="$host"

    remain="$query"
    while [ -n "$remain" ]; do
        case "$remain" in
            *'&'*)
                pair="${remain%%&*}"
                remain="${remain#*&}"
                ;;
            *)
                pair="$remain"
                remain=""
                ;;
        esac
        [ -z "$pair" ] && continue
        key="${pair%%=*}"
        value="${pair#*=}"
        if [ "$key" = "$pair" ]; then
            value=""
        fi
        case "$key" in
            type|network)
                [ -n "$value" ] && network_type="$value"
                ;;
            security)
                [ -n "$value" ] && security_type="$value"
                ;;
            allowInsecure)
                case "$value" in
                    1|true|TRUE|yes|on|enable|enabled)
                        allow_insecure_value="true"
                        ;;
                    0|false|FALSE|no|off|disable|disabled)
                        allow_insecure_value="false"
                        ;;
                esac
                ;;
            sni|peer)
                if [ -n "$value" ]; then
                    server_name="$value"
                fi
                ;;
            tag)
                if [ -n "$value" ]; then
                    fragment="$value"
                fi
                ;;
        esac
    done

    CLIENT_CONNECTION_URL="$url"
    CLIENT_CONNECTION_PASSWORD="$password_part"
    CLIENT_CONNECTION_HOST="$host"
    CLIENT_CONNECTION_PORT="$port_num"
    CLIENT_CONNECTION_SERVER_NAME="$server_name"
    CLIENT_CONNECTION_NETWORK="$network_type"
    CLIENT_CONNECTION_SECURITY="$security_type"
    CLIENT_CONNECTION_ALLOW_INSECURE="$allow_insecure_value"
    CLIENT_CONNECTION_LABEL="$fragment"
    CLIENT_CONNECTION_TAG=$(client_connection_generate_tag "$host" "$port_num")

    return 0
}
