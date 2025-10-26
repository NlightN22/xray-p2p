#!/bin/sh
# shellcheck shell=sh

server_reverse_trim_spaces() {
    printf '%s' "$1" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//'
}

server_reverse_subnet_reset() {
    SERVER_REVERSE_SUBNETS=""
}

server_reverse_subnet_contains() {
    needle="$1"
    case "
$SERVER_REVERSE_SUBNETS
" in
        *"
$needle
"*) return 0 ;;
    esac
    return 1
}

server_reverse_subnet_add() {
    candidate=$(server_reverse_trim_spaces "$1")
    [ -n "$candidate" ] || return 0

    if ! validate_subnet "$candidate"; then
        xray_die "Invalid subnet: $candidate"
    fi

    if server_reverse_subnet_contains "$candidate"; then
        return 0
    fi

    if [ -n "$SERVER_REVERSE_SUBNETS" ]; then
        SERVER_REVERSE_SUBNETS="$SERVER_REVERSE_SUBNETS
$candidate"
    else
        SERVER_REVERSE_SUBNETS="$candidate"
    fi
}

server_reverse_subnet_add_many() {
    input="$1"
    [ -n "$input" ] || return 0

    sanitized=$(printf '%s' "$input" | tr ',' ' ')
    for token in $sanitized; do
        server_reverse_subnet_add "$token"
    done
}

server_reverse_prompt_subnets() {
    if [ -n "$SERVER_REVERSE_SUBNETS" ]; then
        return
    fi

    read_fd=0
    if [ ! -t 0 ]; then
        if [ -r /dev/tty ]; then
            exec 4</dev/tty
            read_fd=4
        else
            return
        fi
    fi

    xray_log "No CIDR subnets supplied; press Enter to skip or provide one per prompt."

    while :; do
        printf 'Enter CIDR subnet for reverse routing (blank to finish): ' >&2
        if [ "$read_fd" -eq 4 ]; then
            IFS= read -r input <&4 || input=""
        else
            IFS= read -r input || input=""
        fi

        trimmed=$(server_reverse_trim_spaces "$input")
        if [ -z "$trimmed" ]; then
            break
        fi

        if ! validate_subnet "$trimmed"; then
            xray_log "Invalid subnet, expected CIDR (e.g. 10.0.102.0/24)."
            continue
        fi

        if server_reverse_subnet_contains "$trimmed"; then
            xray_log "Subnet '$trimmed' already recorded."
            continue
        fi

        server_reverse_subnet_add "$trimmed"
    done

    if [ "$read_fd" -eq 4 ]; then
        exec 4<&-
    fi
}

server_reverse_subnet_json() {
    if [ -z "$SERVER_REVERSE_SUBNETS" ]; then
        printf '[]'
    else
        printf '%s' "$SERVER_REVERSE_SUBNETS" | jq -Rsc 'split("\n") | map(select(length > 0))'
    fi
}

server_reverse_subnet_primary() {
    if [ -z "$SERVER_REVERSE_SUBNETS" ]; then
        return 1
    fi

    printf '%s\n' "$SERVER_REVERSE_SUBNETS" | sed -n '1p'
}

server_reverse_read_server() {
    supplied="$1"
    if [ -n "$supplied" ]; then
        printf '%s' "$supplied"
        return
    fi

    if [ -n "${XRAY_REVERSE_SERVER_ID:-}" ]; then
        printf '%s' "$XRAY_REVERSE_SERVER_ID"
        return
    fi

    if [ ! -t 0 ]; then
        if [ -r /dev/tty ]; then
            exec 5</dev/tty
            read_fd=5
        else
            xray_die "Server identifier argument required; no interactive terminal available"
        fi
    else
        read_fd=0
    fi

    while :; do
        printf 'Enter external server identifier: ' >&2
        if [ "${read_fd:-0}" -eq 5 ]; then
            IFS= read -r input <&5 || input=""
        else
            IFS= read -r input || input=""
        fi
        trimmed=$(server_reverse_trim_spaces "$input")
        if [ -n "$trimmed" ]; then
            if [ "${read_fd:-0}" -eq 5 ]; then
                exec 5<&-
            fi
            printf '%s' "$trimmed"
            return
        fi
        xray_log "Server identifier cannot be empty."
    done
}

server_reverse_validate_server() {
    candidate="$1"
    case "$candidate" in
        ''|*[!A-Za-z0-9._-]*)
            xray_die "Server identifier must contain only letters, digits, dot, underscore, or dash"
            ;;
    esac
}

server_reverse_sanitize_component() {
    printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | sed 's/[^0-9a-z]/-/g; s/-\{2,\}/-/g; s/^-//; s/-$//'
}

server_reverse_resolve_tunnel_id() {
    primary_subnet="$1"
    server_id="$2"
    candidate="${XRAY_REVERSE_TUNNEL_ID:-}"
    sanitized=""

    if [ -n "$candidate" ]; then
        sanitized=$(server_reverse_sanitize_component "$candidate")
    fi
    if [ -n "$sanitized" ]; then
        printf '%s' "$sanitized"
        return
    fi

    subnet_part=$(server_reverse_sanitize_component "$primary_subnet")
    server_part=$(server_reverse_sanitize_component "$server_id")
    [ -n "$subnet_part" ] || subnet_part="nosubnet"
    [ -n "$server_part" ] || server_part="server"
    printf '%s--%s' "$subnet_part" "$server_part"
}
