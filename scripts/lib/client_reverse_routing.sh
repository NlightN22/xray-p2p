#!/bin/sh
# shellcheck shell=sh

client_reverse_ensure_routing_file() {
    routing_file="$1"
    template_local="$2"
    template_remote="$3"

    if [ -f "$routing_file" ]; then
        return
    fi

    if [ -n "$template_remote" ]; then
        xray_seed_file_from_template "$routing_file" "$template_remote" "${template_local:-$template_remote}"
        return
    fi

    if [ -n "$template_local" ]; then
        if resolved_template=$(xray_resolve_local_path "$template_local"); then
            template_path="$resolved_template"
        else
            template_path="$resolved_template"
        fi

        if [ -n "$template_path" ] && [ -r "$template_path" ]; then
            if ! xray_should_replace_file "$routing_file" "XRAY_FORCE_CONFIG"; then
                return
            fi
            dest_dir=$(dirname "$routing_file")
            if [ ! -d "$dest_dir" ]; then
                mkdir -p "$dest_dir" || xray_die "Unable to create directory $dest_dir"
            fi
            if ! cp "$template_path" "$routing_file"; then
                xray_die "Failed to copy template from $template_path"
            fi
            chmod 0644 "$routing_file" 2>/dev/null || true
            xray_log "Seeded $routing_file from local template $template_path"
            return
        fi
    fi

    tmp="$(mktemp 2>/dev/null)"
    if [ -z "$tmp" ]; then
        xray_die "Unable to create temporary file"
    fi
    cat >"$tmp" <<'EOF'
{
    "reverse": {
        "bridges": []
    },
    "routing": {
        "domainStrategy": "AsIs",
        "rules": []
    }
}
EOF
    chmod 0644 "$tmp"
    dest_dir=$(dirname "$routing_file")
    if [ ! -d "$dest_dir" ]; then
        mkdir -p "$dest_dir" || {
            rm -f "$tmp"
            xray_die "Unable to create directory $dest_dir"
        }
    fi
    mv "$tmp" "$routing_file"
    xray_log "Generated default routing config at $routing_file"
}

client_reverse_update_routing() {
    routing_file="$1"
    tunnel_id="$2"
    suffix="$3"
    outbound_tag="$4"

    domain="$tunnel_id$suffix"
    tag="$domain"

    tmp="$(mktemp 2>/dev/null)"
    if [ -z "$tmp" ]; then
        xray_die "Unable to create temporary file"
    fi

    if ! jq \
        --arg domain "$domain" \
        --arg tag "$tag" \
        --arg outbound "$outbound_tag" \
        '
        def ensure_array:
            if . == null then []
            elif type == "array" then .
            else [.] end;

        .reverse = (.reverse // {}) |
        .reverse.bridges = (
            (.reverse.bridges // [])
            | [ .[] | select(.domain != $domain) ]
            + [{ domain: $domain, tag: $tag }]
        ) |
        .routing = (.routing // {}) |
        (.routing.rules // []) as $rules |
        .routing.rules = (
            [ $rules[] | select(
                (
                    (.type == "field" and (.outboundTag == "proxy") and ((.inboundTag | ensure_array | index($tag)) != null) and ((.domain | ensure_array | index("full:" + $domain)) != null))
                    or (.type == "field" and (.outboundTag == "direct") and ((.inboundTag | ensure_array | index($tag)) != null))
                ) | not
            ) ]
            + [
                {
                    type: "field",
                    domain: ["full:" + $domain],
                    inboundTag: [$tag],
                    outboundTag: $outbound
                },
                {
                    type: "field",
                    inboundTag: [$tag],
                    outboundTag: "direct"
                }
            ]
        )
        ' "$routing_file" >"$tmp"; then
        rm -f "$tmp"
        xray_die "Failed to update $routing_file"
    fi

    chmod 0644 "$tmp"
    mv "$tmp" "$routing_file"
    xray_log "Updated $routing_file with reverse proxy entry for $tunnel_id (tag: $tag)"
}

client_reverse_remove_routing() {
    routing_file="$1"
    tunnel_id="$2"
    suffix="$3"

    if [ ! -f "$routing_file" ]; then
        xray_log "Routing file $routing_file not found; skipping removal for tunnel $tunnel_id."
        return
    fi

    domain="$tunnel_id$suffix"
    tag="$domain"

    tmp="$(mktemp 2>/dev/null)"
    if [ -z "$tmp" ]; then
        xray_die "Unable to create temporary file"
    fi

    if ! jq \
        --arg domain "$domain" \
        --arg tag "$tag" \
        '
        def ensure_array:
            if . == null then []
            elif type == "array" then .
            else [.] end;

        .reverse = (.reverse // {}) |
        .reverse.bridges = (
            (.reverse.bridges // [])
            | [ .[] | select(.domain != $domain) ]
        ) |
        .routing = (.routing // {}) |
        .routing.rules = (
            (.routing.rules // [])
            | [ .[] | select(
                not (
                    (.type // "") == "field"
                    and (
                        ((ensure_array(.domain) | index("full:" + $domain)) != null)
                        or ((ensure_array(.inboundTag) | index($tag)) != null)
                    )
                )
            ) ]
        )
        ' "$routing_file" >"$tmp"; then
        rm -f "$tmp"
        xray_die "Failed to update $routing_file while removing tunnel $tunnel_id"
    fi

    chmod 0644 "$tmp"
    mv "$tmp" "$routing_file"
    xray_log "Removed reverse proxy entry for $tunnel_id (tag: $tag) from $routing_file"
}
