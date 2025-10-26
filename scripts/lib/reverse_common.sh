#!/bin/sh
# shellcheck shell=sh

# Shared helpers for reverse tunnel slug normalisation.

if [ "${XRAY_REVERSE_COMMON_LOADED:-0}" = "1" ]; then
    return 0 2>/dev/null || true
fi
XRAY_REVERSE_COMMON_LOADED=1

reverse_trim_spaces() {
    printf '%s' "$1" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//'
}

reverse_normalize_component() {
    printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | sed 's/[^0-9a-z]/-/g; s/-\{2,\}/-/g; s/^-//; s/-$//'
}

reverse_validate_component() {
    local value="$1"
    local original="$2"

    if [ -n "$value" ]; then
        printf '%s' "$value"
        return 0
    fi

    xray_die "Unable to derive reverse tunnel id: '$original' must contain at least one alphanumeric character."
}

reverse_resolve_tunnel_id() {
    local primary_subnet="$1"
    local server_id="$2"
    local override="${3:-${XRAY_REVERSE_TUNNEL_ID:-}}"
    local sanitised=""

    if [ -n "$override" ]; then
        override=$(reverse_trim_spaces "$override")
        override=${override%%[[:space:]]*}
        override=$(printf '%s' "$override" | tr '[:upper:]' '[:lower:]')

        if printf '%s' "$override" | grep -q -- '--'; then
            local prefix=${override%%--*}
            local suffix=${override#*--}

            prefix=$(reverse_normalize_component "$prefix")
            prefix=$(reverse_validate_component "$prefix" "$override")

            suffix=$(reverse_normalize_component "$suffix")
            suffix=$(reverse_validate_component "$suffix" "$override")

            printf '%s--%s' "$prefix" "$suffix"
            return 0
        fi

        sanitised=$(printf '%s' "$override" | sed 's/[^0-9a-z-]/-/g; s/^-//; s/-$//')
        reverse_validate_component "$sanitised" "$override"
        printf '%s' "$sanitised"
        return 0
    fi

    local subnet_part server_part
    subnet_part=$(reverse_normalize_component "$primary_subnet")
    subnet_part=$(reverse_validate_component "$subnet_part" "$primary_subnet")

    server_part=$(reverse_normalize_component "$server_id")
    server_part=$(reverse_validate_component "$server_part" "$server_id")

    printf '%s--%s' "$subnet_part" "$server_part"
}
