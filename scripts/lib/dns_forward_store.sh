#!/bin/sh
# shellcheck shell=sh

dns_forward_store_ensure() {
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
    xray_log "Created DNS forward metadata store at $store_file"
}

dns_forward_store_require() {
    store_file="$1"
    if [ ! -f "$store_file" ]; then
        xray_die "DNS forward metadata file not found: $store_file"
    fi
    if ! jq empty "$store_file" >/dev/null 2>&1; then
        xray_die "Existing $store_file contains invalid JSON."
    fi
}

dns_forward_store_now_iso() {
    date -u '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || date '+%Y-%m-%dT%H:%M:%SZ'
}

dns_forward_store_has() {
    store_file="$1"
    domain_mask="$2"

    jq -e --arg domain_mask "$domain_mask" '
        any(.[]?; (.domain_mask // "") == $domain_mask)
    ' "$store_file" >/dev/null 2>&1
}

dns_forward_store_add() {
    store_file="$1"
    store_dir="$2"
    domain_mask="$3"
    dns_ip="$4"
    listen="$5"
    local_port="$6"
    tag="$7"
    remark="$8"
    rebind="$9"

    dns_forward_store_ensure "$store_file" "$store_dir"

    now=$(dns_forward_store_now_iso)
    tmp="$(mktemp 2>/dev/null)"
    if [ -z "$tmp" ]; then
        xray_die "Unable to create temporary file"
    fi

    if ! jq \
        --arg domain_mask "$domain_mask" \
        --arg dns_ip "$dns_ip" \
        --arg listen "$listen" \
        --arg tag "$tag" \
        --arg remark "$remark" \
        --arg rebind "$rebind" \
        --arg now "$now" \
        --argjson local_port "$local_port" \
        '
        ( . // [] ) as $items
        | reduce $items[] as $item (
            {items: [], updated: false};
            if ($item.domain_mask // "") == $domain_mask then
                {items: .items + [$item + {
                    dns_ip: $dns_ip,
                    listen: $listen,
                    local_port: $local_port,
                    tag: $tag,
                    remark: $remark,
                    rebind: $rebind,
                    updated_at: $now
                }], updated: true}
            else
                {items: .items + [$item], updated: .updated}
            end
        ) as $state
        | if $state.updated then
            $state.items
          else
            $state.items + [{
                domain_mask: $domain_mask,
                dns_ip: $dns_ip,
                listen: $listen,
                local_port: $local_port,
                tag: $tag,
                remark: $remark,
                rebind: $rebind,
                created_at: $now,
                updated_at: $now,
                notes: ""
            }]
          end
        ' "$store_file" >"$tmp"; then
        rm -f "$tmp"
        xray_die "Failed to update $store_file"
    fi

    chmod 0600 "$tmp" 2>/dev/null || true
    mv "$tmp" "$store_file"
}

dns_forward_store_remove() {
    store_file="$1"
    domain_mask="$2"

    tmp="$(mktemp 2>/dev/null)"
    if [ -z "$tmp" ]; then
        xray_die "Unable to create temporary file"
    fi

    if ! jq --arg domain_mask "$domain_mask" '
        ( . // [] ) | [ .[] | select((.domain_mask // "") != $domain_mask) ]
    ' "$store_file" >"$tmp"; then
        rm -f "$tmp"
        xray_die "Failed to update $store_file while removing $domain_mask"
    fi

    chmod 0600 "$tmp" 2>/dev/null || true
    mv "$tmp" "$store_file"
}

dns_forward_store_get() {
    store_file="$1"
    domain_mask="$2"

    dns_forward_store_require "$store_file"

    jq -er --arg domain_mask "$domain_mask" '
        (.[]? | select((.domain_mask // "") == $domain_mask)) // empty
    ' "$store_file"
}

dns_forward_store_print_table() {
    store_file="$1"

    dns_forward_store_require "$store_file"

    output=$(jq -r '
        (["domain_mask","dns_ip","local_port","tag","created_at"] | @tsv),
        (.[] | [(.domain_mask // "-"), (.dns_ip // "-"), (.local_port // "-"), (.tag // "-"), (.created_at // "-")] | @tsv)
    ' "$store_file")

    if command -v column >/dev/null 2>&1; then
        printf '%s\n' "$output" | column -t -s '	'
    else
        printf '%s\n' "$output"
    fi
}
