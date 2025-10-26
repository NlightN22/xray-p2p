#!/bin/sh
# shellcheck shell=sh

client_reverse_store_ensure() {
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
    xray_log "Created client reverse metadata store at $store_file"
}

client_reverse_store_require() {
    store_file="$1"
    if [ ! -f "$store_file" ]; then
        xray_die "Client reverse metadata file not found: $store_file"
    fi
    if ! jq empty "$store_file" >/dev/null 2>&1; then
        xray_die "Existing $store_file contains invalid JSON."
    fi
}

client_reverse_store_now_iso() {
    date -u '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || date '+%Y-%m-%dT%H:%M:%SZ'
}

client_reverse_store_has() {
    store_file="$1"
    key="$2"

    jq -e --arg key "$key" '
        any(.[]?; (.tunnel_id // "") == $key)
    ' "$store_file" >/dev/null 2>&1
}

client_reverse_store_add() {
    store_file="$1"
    store_dir="$2"
    tunnel_id="$3"
    domain="$4"
    tag="$5"
    server_id="$6"
    outbound_tag="$7"

    client_reverse_store_ensure "$store_file" "$store_dir"

    if client_reverse_store_has "$store_file" "$tunnel_id"; then
        xray_die "Client reverse '$tunnel_id' already exists in $store_file"
    fi

    created_at=$(client_reverse_store_now_iso)
    tmp="$(mktemp 2>/dev/null)"
    if [ -z "$tmp" ]; then
        xray_die "Unable to create temporary file"
    fi

    if ! jq \
        --arg tunnel_id "$tunnel_id" \
        --arg domain "$domain" \
        --arg tag "$tag" \
        --arg server_id "$server_id" \
        --arg outbound "$outbound_tag" \
        --arg created_at "$created_at" \
        '
        (. // []) + [{
            tunnel_id: $tunnel_id,
            domain: $domain,
            tag: $tag,
            server_id: $server_id,
            outbound_tag: $outbound,
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

client_reverse_store_remove() {
    store_file="$1"
    key="$2"

    tmp="$(mktemp 2>/dev/null)"
    if [ -z "$tmp" ]; then
        xray_die "Unable to create temporary file"
    fi

    if ! jq --arg key "$key" '
        ( . // [] ) | [ .[] | select((.tunnel_id // "") != $key) ]
    ' "$store_file" >"$tmp"; then
        rm -f "$tmp"
        xray_die "Failed to update $store_file while removing $key"
    fi

    chmod 0600 "$tmp" 2>/dev/null || true
    mv "$tmp" "$store_file"
}

client_reverse_store_print_table() {
    store_file="$1"

    output=$(jq -r '
        (["tunnel_id","domain","server_id","outbound_tag","created_at"] | @tsv),
        (.[] | [(.tunnel_id // "-"), (.domain // "-"), (.server_id // "-"), (.outbound_tag // "-"), (.created_at // "-")] | @tsv)
    ' "$store_file")

    if command -v column >/dev/null 2>&1; then
        printf '%s\n' "$output" | column -t -s '	'
    else
        printf '%s\n' "$output"
    fi
}
