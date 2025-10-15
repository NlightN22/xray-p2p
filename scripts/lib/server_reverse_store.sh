#!/bin/sh
# shellcheck shell=sh

server_reverse_store_ensure() {
    store_file="$1"
    store_dir="$2"

    if [ -f "$store_file" ]; then
        if ! jq empty "$store_file" >/dev/null 2>&1; then
            xray_die "Existing $store_file contains invalid JSON."
        fi
        return
    fi

    if [ ! -d "$store_dir" ]; then
        mkdir -p "$store_dir" || xray_die "Unable to create directory $store_dir"
    fi

    tmp="$(mktemp 2>/dev/null)"
    if [ -z "$tmp" ]; then
        xray_die "Unable to create temporary file"
    fi
    printf '[]\n' >"$tmp"
    chmod 0600 "$tmp" 2>/dev/null || true
    mv "$tmp" "$store_file"
    xray_log "Created tunnel metadata store at $store_file"
}

server_reverse_store_require() {
    store_file="$1"
    if [ ! -f "$store_file" ]; then
        xray_die "Tunnel metadata file not found: $store_file"
    fi
    if ! jq empty "$store_file" >/dev/null 2>&1; then
        xray_die "Existing $store_file contains invalid JSON."
    fi
}

server_reverse_store_now_iso() {
    date -u '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || date '+%Y-%m-%dT%H:%M:%SZ'
}

server_reverse_store_has() {
    store_file="$1"
    username="$2"

    jq -e --arg username "$username" '
        any(.[]?; (.username // "") == $username)
    ' "$store_file" >/dev/null 2>&1
}

server_reverse_store_add() {
    store_file="$1"
    store_dir="$2"
    username="$3"
    domain="$4"
    tag="$5"
    subnet_json="$6"

    server_reverse_store_ensure "$store_file" "$store_dir"

    if server_reverse_store_has "$store_file" "$username"; then
        return 0
    fi

    created_at=$(server_reverse_store_now_iso)
    tmp="$(mktemp 2>/dev/null)"
    if [ -z "$tmp" ]; then
        xray_die "Unable to create temporary file"
    fi
    if ! jq \
        --arg username "$username" \
        --arg domain "$domain" \
        --arg tag "$tag" \
        --arg created_at "$created_at" \
        --argjson subnets "$subnet_json" \
        '
        (. // []) + [{
            username: $username,
            domain: $domain,
            tag: $tag,
            subnets: $subnets,
            created_at: $created_at,
            updated_at: $created_at,
            notes: ""
        }]
        ' "$store_file" >"$tmp"; then
        rm -f "$tmp"
        xray_die "Failed to update $store_file"
    fi

    chmod 0600 "$tmp" 2>/dev/null || true
    mv "$tmp" "$store_file"
}

server_reverse_store_remove() {
    store_file="$1"
    username="$2"

    tmp="$(mktemp 2>/dev/null)"
    if [ -z "$tmp" ]; then
        xray_die "Unable to create temporary file"
    fi

    if ! jq --arg username "$username" '
        ( . // [] ) | [ .[] | select((.username // "") != $username) ]
    ' "$store_file" >"$tmp"; then
        rm -f "$tmp"
        xray_die "Failed to update $store_file while removing $username"
    fi

    chmod 0600 "$tmp" 2>/dev/null || true
    mv "$tmp" "$store_file"
}

server_reverse_store_print_table() {
    store_file="$1"

    output=$(jq -r '
        (["username","domain","subnets","created_at"] | @tsv),
        (.[] | [(.username // "-"), (.domain // "-"), ((.subnets // []) | join(",")), (.created_at // "-")] | @tsv)
    ' "$store_file")

    if command -v column >/dev/null 2>&1; then
        printf '%s\n' "$output" | column -t -s '	'
    else
        printf '%s\n' "$output"
    fi
}
