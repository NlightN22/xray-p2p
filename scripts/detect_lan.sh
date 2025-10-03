#!/bin/sh

set -eu

log() {
    printf '%s\n' "$*"
}

die() {
    printf 'Error: %s\n' "$*" >&2
    exit 1
}

require_cmd() {
    cmd="$1"
    if ! command -v "$cmd" >/dev/null 2>&1; then
        die "Required command '$cmd' not found"
    fi
}

is_private_ipv4() {
    addr="$1"
    IFS_SAVE="$IFS"
    IFS='.' read -r o1 o2 o3 o4 <<EOF_ADDR
$addr
EOF_ADDR
    IFS="$IFS_SAVE"

    case "$o1" in
        10) return 0 ;;
        192)
            [ "$o2" = "168" ] && return 0
            ;;
        172)
            if [ "$o2" -ge 16 ] && [ "$o2" -le 31 ]; then
                return 0
            fi
            ;;
        100)
            if [ "$o2" -ge 64 ] && [ "$o2" -le 127 ]; then
                return 0
            fi
            ;;
    esac
    return 1
}

first_ipv4_from_list() {
    if [ -z "$1" ]; then
        return 1
    fi
    set -- $1
    printf '%s' "$1"
}

get_ip_for_iface() {
    iface="$1"
    ip -o -4 addr show scope global dev "$iface" 2>/dev/null | awk 'NR==1 {print $4}' | sed 's#/.*##' | head -n1
}

pick_private_iface() {
    ip -o -4 addr show scope global 2>/dev/null | awk '
        function score(name) {
            if (name == "br-lan") return 5;
            if (name ~ /lan/) return 4;
            if (name ~ /^br-/) return 3;
            return 1;
        }
        function is_private(o1, o2) {
            if (o1 == 10) return 1;
            if (o1 == 192 && o2 == 168) return 1;
            if (o1 == 172 && o2 >= 16 && o2 <= 31) return 1;
            if (o1 == 100 && o2 >= 64 && o2 <= 127) return 1;
            return 0;
        }
        {
            dev=$2; sub(":", "", dev);
            split($4, addr, "/");
            split(addr[1], oct, ".");
            if (is_private(oct[1] + 0, oct[2] + 0)) {
                printf "%d %s %s\n", score(dev), dev, addr[1];
            }
        }
    ' | sort -rn -k1,1 | head -n1
}

require_cmd ip

LAN_IFACE=""
LAN_IP=""

if command -v uci >/dev/null 2>&1; then
    val=$(uci -q get network.lan.device 2>/dev/null || true)
    [ -z "$val" ] && val=$(uci -q get network.lan.ifname 2>/dev/null || true)
    if [ -n "$val" ]; then
        LAN_IFACE=$(first_ipv4_from_list "$val" || true)
    fi

    val_ip=$(uci -q get network.lan.ipaddr 2>/dev/null || true)
    if [ -n "$val_ip" ]; then
        LAN_IP=$(first_ipv4_from_list "$val_ip" || true)
    fi
fi

if [ -z "$LAN_IFACE" ] || [ -z "$LAN_IP" ]; then
    if command -v ubus >/dev/null 2>&1 && command -v jsonfilter >/dev/null 2>&1; then
        status=$(ubus call network.interface.lan status 2>/dev/null || true)
        if [ -n "$status" ]; then
            if [ -z "$LAN_IFACE" ]; then
                iface=$(printf '%s\n' "$status" | jsonfilter -e '@.l3_device' 2>/dev/null || true)
                [ -z "$iface" ] && iface=$(printf '%s\n' "$status" | jsonfilter -e '@.device' 2>/dev/null || true)
                if [ -n "$iface" ]; then
                    LAN_IFACE="$iface"
                fi
            fi
            if [ -z "$LAN_IP" ]; then
                addr=$(printf '%s\n' "$status" | jsonfilter -e '@["ipv4-address"][0].address' 2>/dev/null || true)
                if [ -n "$addr" ]; then
                    LAN_IP="$addr"
                fi
            fi
        fi
    fi
fi

if [ -n "$LAN_IFACE" ] && [ -z "$LAN_IP" ]; then
    ip_candidate=$(get_ip_for_iface "$LAN_IFACE")
    if [ -n "$ip_candidate" ]; then
        LAN_IP="$ip_candidate"
    fi
fi

if [ -z "$LAN_IFACE" ] || [ -z "$LAN_IP" ]; then
    pick=$(pick_private_iface || true)
    if [ -n "$pick" ]; then
        set -- $pick
        if [ -z "$LAN_IFACE" ] && [ -n "${2:-}" ]; then
            LAN_IFACE="$2"
        fi
        if [ -z "$LAN_IP" ] && [ -n "${3:-}" ]; then
            LAN_IP="$3"
        fi
    fi
fi

if [ -z "$LAN_IFACE" ] && [ -n "$LAN_IP" ]; then
    candidate=$(ip -o -4 addr show scope global 2>/dev/null | awk -v target="$LAN_IP" '
        {
            dev=$2; sub(":", "", dev);
            split($4, addr, "/");
            if (addr[1] == target) {
                print dev;
                exit;
            }
        }
    ')
    if [ -n "$candidate" ]; then
        LAN_IFACE="$candidate"
    fi
fi

if [ -z "$LAN_IFACE" ] && [ -z "$LAN_IP" ]; then
    die "Unable to detect LAN interface and IP"
fi

if [ -z "$LAN_IP" ]; then
    die "Detected LAN interface '$LAN_IFACE' but no IPv4 address is configured"
fi

if ! is_private_ipv4 "$LAN_IP"; then
    log "Warning: detected LAN IPv4 $LAN_IP is not in a private range"
fi

log "LAN interface: $LAN_IFACE"
log "LAN IPv4: $LAN_IP"
