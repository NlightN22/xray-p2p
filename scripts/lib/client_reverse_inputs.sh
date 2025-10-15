#!/bin/sh
# shellcheck shell=sh

client_reverse_trim_spaces() {
    printf '%s' "$1" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//'
}

client_reverse_read_username() {
    value="$1"
    if [ -n "$value" ]; then
        printf '%s' "$value"
        return 0
    fi

    if [ -n "${XRAY_REVERSE_USER:-}" ]; then
        printf '%s' "$XRAY_REVERSE_USER"
        return 0
    fi

    read_fd=0
    if [ ! -t 0 ]; then
        if [ -r /dev/tty ]; then
            exec 3</dev/tty
            read_fd=3
        else
            xray_die "Username argument required; no interactive terminal available"
        fi
    fi

    while :; do
        printf 'Enter XRAY username: ' >&2
        if [ "$read_fd" -eq 3 ]; then
            IFS= read -r input <&3 || input=""
        else
            IFS= read -r input || input=""
        fi
        if [ -n "$input" ]; then
            if [ "$read_fd" -eq 3 ]; then
                exec 3<&-
            fi
            printf '%s' "$input"
            return 0
        fi
        xray_log "Username cannot be empty."
    done
}

client_reverse_validate_username() {
    candidate="$1"
    case "$candidate" in
        ''|*[!A-Za-z0-9._-]*)
            xray_die "Username must contain only letters, digits, dot, underscore, or dash"
            ;;
    esac
}
