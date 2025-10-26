#!/bin/sh
# shellcheck shell=sh

server_reverse_ensure_routing_file() {
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
        "portals": []
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

server_reverse_update_routing() {
    routing_file="$1"
    tunnel_id="$2"
    suffix="$3"
    subnet_json="$4"
    server_id="$5"

    domain="$tunnel_id$suffix"
    tag="$domain"

    tmp="$(mktemp 2>/dev/null)"
    if [ -z "$tmp" ]; then
        xray_die "Unable to create temporary file"
    fi

    if ! jq \
        --arg domain "$domain" \
        --arg tag "$tag" \
        --argjson subnets "$subnet_json" \
        '
        def ensure_array:
            if . == null then []
            elif type == "array" then .
            else [.] end;

        def has_domain_match($rule_domain):
            ($rule_domain | ensure_array | index("full:" + $domain)) != null;

        def has_subnet_match($rule_ip):
            if ($subnets | length) == 0 then
                false
            else
                ($rule_ip | ensure_array) as $ips
                | any($subnets[]; ($ips | index(.)) != null)
            end;

        .reverse = (.reverse // {}) |
        .reverse.portals = (
            (.reverse.portals // [])
            | [ .[] | select(.domain != $domain) ]
            + [{ domain: $domain, tag: $tag }]
        ) |
        .routing = (.routing // {}) |
        (.routing.rules // []) as $rules |
        .routing.rules = (
            [ $rules[] | select((
                (.outboundTag == $tag and (has_domain_match(.domain) or has_subnet_match(.ip))) | not
            )) ]
            + [{
                type: "field",
                domain: ["full:" + $domain],
                outboundTag: $tag
            }]
            + (if ($subnets | length) > 0 then [
                {
                    type: "field",
                    ip: $subnets,
                    outboundTag: $tag
                }
            ] else [] end)
        )
        ' "$routing_file" >"$tmp"; then
        rm -f "$tmp"
        xray_die "Failed to update $routing_file"
    fi

    chmod 0644 "$tmp"
    mv "$tmp" "$routing_file"
    xray_log "Updated $routing_file with reverse proxy entry for tunnel $tunnel_id (server $server_id, tag: $tag)"
}

server_reverse_remove_routing() {
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
        .reverse = (.reverse // {}) |
        .reverse.portals = (
            (.reverse.portals // [])
            | [ .[] | select(.domain != $domain) ]
        ) |
        .routing = (.routing // {}) |
        .routing.rules = (
            (.routing.rules // [])
            | [ .[] | select((.outboundTag // "") != $tag) ]
        )
        ' "$routing_file" >"$tmp"; then
        rm -f "$tmp"
        xray_die "Failed to update $routing_file while removing tunnel $tunnel_id"
    fi

    chmod 0644 "$tmp"
    mv "$tmp" "$routing_file"
    xray_log "Removed reverse proxy entry for tunnel $tunnel_id (tag: $tag) from $routing_file"
}
