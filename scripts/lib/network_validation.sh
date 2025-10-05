#!/bin/sh

# Network validation helpers shared across XRAY management scripts.

if [ "${XRAY_NETWORK_VALIDATION_LOADED:-0}" = "1" ]; then
    return 0 2>/dev/null || true
fi
XRAY_NETWORK_VALIDATION_LOADED=1

validate_ipv4() {
    addr="${1:-}"
    old_ifs=$IFS
    IFS='.'
    # shellcheck disable=SC2086
    set -- $addr
    IFS=$old_ifs

    if [ "$#" -ne 4 ]; then
        return 1
    fi

    for octet in "$@"; do
        case "$octet" in
            ''|*[!0-9]*)
                return 1
                ;;
        esac
        if [ "$octet" -lt 0 ] || [ "$octet" -gt 255 ]; then
            return 1
        fi
    done

    return 0
}

validate_subnet() {
    subnet="${1:-}"
    case "$subnet" in
        */*)
            :
            ;;
        *)
            return 1
            ;;
    esac

    addr="${subnet%/*}"
    prefix="${subnet#*/}"

    if ! validate_ipv4 "$addr"; then
        return 1
    fi

    case "$prefix" in
        ''|*[!0-9]*)
            return 1
            ;;
    esac

    if [ "$prefix" -lt 0 ] || [ "$prefix" -gt 32 ]; then
        return 1
    fi

    return 0
}
