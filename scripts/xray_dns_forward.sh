#!/bin/sh

set -eu

XRAY_INBOUND_FILE="/etc/xray/inbounds.json"
LISTEN_ADDRESS="127.0.0.1"
BASE_LOCAL_PORT=53331
DNSMASQ_SECTION="dhcp.@dnsmasq[0]"
DNSMASQ_SERVICE="/etc/init.d/dnsmasq"
XRAY_SERVICE="/etc/init.d/xray"
DNS_REMARK_PREFIX="dns-forward:"
DNS_TAG_PREFIX="in_dns_"

log() {
    printf '%s\n' "$*"
}

die() {
    printf 'Error: %s\n' "$*" >&2
    exit 1
}

require_cmd() {
    if ! command -v "$1" >/dev/null 2>&1; then
        die "Required command '$1' not found. Install it before running this script."
    fi
}

trim() {
    printf '%s' "$1" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//'
}

validate_ipv4() {
    local addr="$1" octet old_ifs
    old_ifs=$IFS
    IFS=.
    set -- $addr
    IFS=$old_ifs
    if [ "$#" -ne 4 ]; then
        return 1
    fi
    for octet in "$@"; do
        case "$octet" in
            ''|*[!0-9]*) return 1 ;;
        esac
        if [ "$octet" -lt 0 ] || [ "$octet" -gt 255 ]; then
            return 1
        fi
    done
    return 0
}

validate_domain_mask() {
    case "$1" in
        ''|*[!A-Za-z0-9.*-]*)
            return 1
            ;;
    esac
    case "$1" in
        *.*)
            return 0
            ;;
    esac
    return 1
}

sanitize_for_tag() {
    printf '%s' "$1" | tr 'A-Z' 'a-z' | sed 's/[^0-9a-z]/_/g'
}

require_cmd jq
require_cmd uci
require_cmd sed
require_cmd cmp
require_cmd sort
require_cmd grep

if [ ! -f "$XRAY_INBOUND_FILE" ]; then
    die "XRAY inbound file $XRAY_INBOUND_FILE not found"
fi

if [ "$#" -gt 2 ]; then
    die "Usage: xray_dns_forward.sh [DOMAIN_MASK] [DNS_IP]"
fi

domain_mask="${1:-}"
dns_ip="${2:-}"

if [ -z "$domain_mask" ]; then
    if [ -t 0 ]; then
        printf 'Enter domain mask (e.g. *.corp.test.com): '
        IFS= read -r domain_mask
    elif [ -r /dev/tty ]; then
        printf 'Enter domain mask (e.g. *.corp.test.com): '
        IFS= read -r domain_mask </dev/tty
    else
        die "Domain mask argument is required"
    fi
fi

domain_mask=$(trim "$domain_mask")
if ! validate_domain_mask "$domain_mask"; then
    die "Domain mask must contain only letters, digits, dots, dashes, or asterisks (example: *.corp.test.com)"
fi

if [ -z "$dns_ip" ]; then
    if [ -t 0 ]; then
        printf 'Enter upstream DNS IP: '
        IFS= read -r dns_ip
    elif [ -r /dev/tty ]; then
        printf 'Enter upstream DNS IP: '
        IFS= read -r dns_ip </dev/tty
    else
        die "Upstream DNS IP argument is required"
    fi
fi

dns_ip=$(trim "$dns_ip")
if ! validate_ipv4 "$dns_ip"; then
    die "Upstream DNS IP must be a valid IPv4 address"
fi

case "$domain_mask" in
    '*.'*) base_domain=${domain_mask#*.} ;;
    .*) base_domain=${domain_mask#.} ;;
    *) base_domain=$domain_mask ;;
esac

base_domain=${base_domain#.}
case "$base_domain" in
    ''|*'*'*)
        die "Cannot derive base domain from mask '$domain_mask'"
        ;;
esac

remark="$DNS_REMARK_PREFIX$domain_mask"
rebind_value="$base_domain"

existing_entry=$(jq -r --arg remark "$remark" '
    (.inbounds[]? | select((.remark // "") == $remark) | [(.tag // ""), (.port // empty)] | @tsv)
' "$XRAY_INBOUND_FILE" | head -n 1)

existing_tag=""
existing_port=""
if [ -n "$existing_entry" ]; then
    set -- $existing_entry
    existing_tag="${1:-}"
    existing_port="${2:-}"
fi

all_ports=$(jq -r '.inbounds[]? | .port | select(type == "number")' "$XRAY_INBOUND_FILE" 2>/dev/null | sort -n)
existing_tags=$(jq -r '.inbounds[]? | .tag // empty' "$XRAY_INBOUND_FILE" 2>/dev/null)

if [ -n "$existing_port" ]; then
    local_port="$existing_port"
else
    candidate=$BASE_LOCAL_PORT
    while echo "$all_ports" | grep -qx "$candidate"; do
        candidate=$((candidate + 1))
        if [ "$candidate" -gt 65535 ]; then
            die "No free ports available from $BASE_LOCAL_PORT to 65535"
        fi
    done
    local_port="$candidate"
fi

server_value="/$domain_mask/$LISTEN_ADDRESS#$local_port"

if [ -n "$existing_tag" ]; then
    tag="$existing_tag"
else
    base_tag="$DNS_TAG_PREFIX$(sanitize_for_tag "$domain_mask")"
    tag="$base_tag"
    suffix=1
    while echo "$existing_tags" | grep -Fx "$tag" >/dev/null 2>&1; do
        tag="${base_tag}_${suffix}"
        suffix=$((suffix + 1))
    done
fi

if [ -n "$existing_port" ]; then
    log "Updating DNS forwarding for $domain_mask (port $local_port)"
else
    log "Adding DNS forwarding for $domain_mask on port $local_port"
fi

log "Upstream DNS IP: $dns_ip"

tmp_inbound=$(mktemp)
trap 'rm -f "$tmp_inbound"' EXIT

if ! jq --arg listen "$LISTEN_ADDRESS" \
        --arg remark "$remark" \
        --arg tag "$tag" \
        --arg address "$dns_ip" \
        --argjson port "$local_port" '
    def dnsInbound:
        {
            tag: $tag,
            remark: $remark,
            listen: $listen,
            port: $port,
            protocol: "dokodemo-door",
            settings: {
                address: $address,
                port: 53,
                network: "tcp,udp",
                followRedirect: false
            }
        };
    if (.inbounds | type) != "array" then
        error("inbounds must be an array")
    else
        (reduce .inbounds[]? as $item
            ({items: [], found: false};
                if ($item.remark // "") == $remark then
                    if .found then
                        {items: .items, found: true}
                    else
                        {items: .items + [dnsInbound], found: true}
                    end
                else
                    {items: .items + [$item], found: .found}
                end
            )) as $state
        | if $state.found then
            .inbounds = $state.items
          else
            .inbounds = ($state.items + [dnsInbound])
          end
    end
' "$XRAY_INBOUND_FILE" >"$tmp_inbound"; then
    die "Failed to update $XRAY_INBOUND_FILE"
fi

inbound_changed=0
if ! cmp -s "$tmp_inbound" "$XRAY_INBOUND_FILE"; then
    chmod 0644 "$tmp_inbound"
    mv "$tmp_inbound" "$XRAY_INBOUND_FILE"
    trap - EXIT
    inbound_changed=1
    log "Updated $XRAY_INBOUND_FILE"
else
    rm -f "$tmp_inbound"
    trap - EXIT
    log "No changes required in $XRAY_INBOUND_FILE"
fi

uci_changed=0
servers_output=$(uci -q show "$DNSMASQ_SECTION.server" 2>/dev/null || true)
server_present=0

if [ -n "$servers_output" ]; then
    while IFS= read -r line; do
        value=${line#*=}
        value=${value%\'}
        value=${value#\'}
        value=${value%\"}
        value=${value#\"}
        case "$value" in
            "/$domain_mask/"*)
                if [ "$value" = "$server_value" ]; then
                    if [ "$server_present" -eq 0 ]; then
                        server_present=1
                    else
                        uci del_list "$DNSMASQ_SECTION.server=$value"
                        uci_changed=1
                    fi
                else
                    uci del_list "$DNSMASQ_SECTION.server=$value"
                    uci_changed=1
                fi
                ;;
        esac
    done <<EOF
$servers_output
EOF
fi

if [ "$server_present" -eq 0 ]; then
    uci add_list "$DNSMASQ_SECTION.server"="$server_value"
    uci_changed=1
    log "Set dnsmasq server entry: $server_value"
else
    log "dnsmasq server entry already set"
fi

rebind_output=$(uci -q show "$DNSMASQ_SECTION.rebind_domain" 2>/dev/null || true)
rebind_present=0

if [ -n "$rebind_output" ]; then
    while IFS= read -r line; do
        value=${line#*=}
        value=${value%\'}
        value=${value#\'}
        value=${value%\"}
        value=${value#\"}
        if [ "$value" = "$rebind_value" ]; then
            if [ "$rebind_present" -eq 0 ]; then
                rebind_present=1
            else
                uci del_list "$DNSMASQ_SECTION.rebind_domain=$value"
                uci_changed=1
            fi
        fi
    done <<EOF
$rebind_output
EOF
fi

if [ "$rebind_present" -eq 0 ]; then
    uci add_list "$DNSMASQ_SECTION.rebind_domain"="$rebind_value"
    uci_changed=1
    log "Set dnsmasq rebind_domain entry: $rebind_value"
else
    log "dnsmasq rebind_domain entry already set"
fi

if [ "$uci_changed" -eq 1 ]; then
    uci commit dhcp
    if [ -x "$DNSMASQ_SERVICE" ]; then
        if "$DNSMASQ_SERVICE" restart >/dev/null 2>&1; then
            log "dnsmasq restarted"
        else
            log "dnsmasq restart failed; please restart manually"
        fi
    else
        log "dnsmasq service script not found at $DNSMASQ_SERVICE"
    fi
else
    log "dnsmasq configuration already up to date"
fi

if [ "$inbound_changed" -eq 1 ]; then
    if [ -x "$XRAY_SERVICE" ]; then
        if "$XRAY_SERVICE" restart >/dev/null 2>&1; then
            log "xray restarted"
        else
            log "xray restart failed; please restart manually"
        fi
    else
        log "xray service script not found at $XRAY_SERVICE"
    fi
else
    log "xray configuration already up to date"
fi

log "Forwarding ready: $domain_mask -> $dns_ip via 127.0.0.1#$local_port"
