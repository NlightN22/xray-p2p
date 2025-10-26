#!/bin/sh
# shellcheck shell=sh

client_reverse_trim_spaces() {
    printf '%s' "$1" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//'
}

client_reverse_read_server() {
    supplied="$1"
    if [ -n "$supplied" ]; then
        printf '%s' "$supplied"
        return
    fi

    if [ -n "${XRAY_REVERSE_SERVER_ID:-}" ]; then
        printf '%s' "$XRAY_REVERSE_SERVER_ID"
        return
    fi

    read_fd=0
    if [ ! -t 0 ]; then
        if [ -r /dev/tty ]; then
            exec 3</dev/tty
            read_fd=3
        else
            xray_die "Server identifier argument required; no interactive terminal available"
        fi
    fi

    while :; do
        printf 'Enter external server identifier: ' >&2
        if [ "$read_fd" -eq 3 ]; then
            IFS= read -r input <&3 || input=""
        else
            IFS= read -r input || input=""
        fi
        trimmed=$(client_reverse_trim_spaces "$input")
        if [ -n "$trimmed" ]; then
            if [ "$read_fd" -eq 3 ]; then
                exec 3<&-
            fi
            printf '%s' "$trimmed"
            return
        fi
        xray_log "Server identifier cannot be empty."
    done
}

client_reverse_validate_server() {
    candidate="$1"
    case "$candidate" in
        ''|*[!A-Za-z0-9._-]*)
            xray_die "Server identifier must contain only letters, digits, dot, underscore, or dash"
            ;;
    esac
}

client_reverse_sanitize_component() {
    printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | sed 's/[^0-9a-z]/-/g; s/-\{2,\}/-/g; s/^-//; s/-$//'
}

client_reverse_resolve_tunnel_id() {
    primary_subnet="$1"
    server_id="$2"
    candidate="${XRAY_REVERSE_TUNNEL_ID:-}"
    sanitized=""

    if [ -n "$candidate" ]; then
        sanitized=$(client_reverse_sanitize_component "$candidate")
    fi
    if [ -n "$sanitized" ]; then
        printf '%s' "$sanitized"
        return
    fi

    subnet_part=$(client_reverse_sanitize_component "$primary_subnet")
    server_part=$(client_reverse_sanitize_component "$server_id")
    [ -n "$subnet_part" ] || subnet_part="nosubnet"
    [ -n "$server_part" ] || server_part="server"
    printf '%s--%s' "$subnet_part" "$server_part"
}
