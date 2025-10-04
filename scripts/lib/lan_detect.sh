#!/bin/sh

# Library module to discover LAN interface/IP heuristically.
# Requires helper functions from common.sh to be available.

xray_lan_warn() {
    printf 'Warning: %s\n' "$*" >&2
}

xray_lan_is_private_ipv4() {
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

xray_lan_first_ipv4_from_list() {
    if [ -z "$1" ]; then
        return 1
    fi
    set -- $1
    printf '%s' "$1"
}

xray_lan_get_ip_for_iface() {
    iface="$1"
    ip -o -4 addr show scope global dev "$iface" 2>/dev/null | awk 'NR==1 {print $4}' | sed 's#/.*##' | head -n1
}

xray_lan_pick_private_iface() {
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

xray_lan_detect_main() {
    xray_require_cmd ip

    LAN_IFACE=""
    LAN_IP=""

    if command -v uci >/dev/null 2>&1; then
        val=$(uci -q get network.lan.device 2>/dev/null || printf '')
        if [ -z "$val" ]; then
            alt=$(uci -q get network.lan.ifname 2>/dev/null || printf '')
            if [ -n "$alt" ]; then
                val="$alt"
            else
                xray_lan_warn "UCI options network.lan.device/ifname are not set"
            fi
        fi
        if [ -n "$val" ]; then
            LAN_IFACE=$(xray_lan_first_ipv4_from_list "$val" || true)
        fi

        val_ip=$(uci -q get network.lan.ipaddr 2>/dev/null || printf '')
        if [ -n "$val_ip" ]; then
            LAN_IP=$(xray_lan_first_ipv4_from_list "$val_ip" || true)
        else
            xray_lan_warn "UCI option network.lan.ipaddr is not set"
        fi
    else
        xray_lan_warn "uci command not found; skipping UCI hints"
    fi

    if [ -z "$LAN_IFACE" ] || [ -z "$LAN_IP" ]; then
        if command -v ubus >/dev/null 2>&1 && command -v jsonfilter >/dev/null 2>&1; then
            status=$(ubus call network.interface.lan status 2>/dev/null || printf '')
            if [ -n "$status" ]; then
                if [ -z "$LAN_IFACE" ]; then
                    iface=$(printf '%s\n' "$status" | jsonfilter -e '@.l3_device' 2>/dev/null || true)
                    [ -z "$iface" ] && iface=$(printf '%s\n' "$status" | jsonfilter -e '@.device' 2>/dev/null || true)
                    if [ -n "$iface" ]; then
                        LAN_IFACE="$iface"
                    else
                        xray_lan_warn "ubus status missing l3_device/device"
                    fi
                fi
                if [ -z "$LAN_IP" ]; then
                    addr=$(printf '%s\n' "$status" | jsonfilter -e '@["ipv4-address"][0].address' 2>/dev/null || true)
                    if [ -n "$addr" ]; then
                        LAN_IP="$addr"
                    else
                        xray_lan_warn "ubus status missing IPv4 address"
                    fi
                fi
            else
                xray_lan_warn "ubus returned no data for network.interface.lan"
            fi
        else
            if ! command -v ubus >/dev/null 2>&1; then
                xray_lan_warn "ubus command not found; skipping runtime interface status"
            fi
            if ! command -v jsonfilter >/dev/null 2>&1; then
                xray_lan_warn "jsonfilter command not found; cannot parse ubus output"
            fi
        fi
    fi

    if [ -n "$LAN_IFACE" ] && [ -z "$LAN_IP" ]; then
        ip_candidate=$(xray_lan_get_ip_for_iface "$LAN_IFACE")
        if [ -n "$ip_candidate" ]; then
            LAN_IP="$ip_candidate"
        fi
    fi

    if [ -z "$LAN_IFACE" ] || [ -z "$LAN_IP" ]; then
        pick=$(xray_lan_pick_private_iface || true)
        if [ -n "$pick" ]; then
            set -- $pick
            if [ -z "$LAN_IFACE" ] && [ -n "${2:-}" ]; then
                LAN_IFACE="$2"
            fi
            if [ -z "$LAN_IP" ] && [ -n "${3:-}" ]; then
                LAN_IP="$3"
            fi
        else
            xray_lan_warn "Failed to infer LAN candidate from ip address list"
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
        xray_die "Unable to detect LAN interface and IP"
    fi

    if [ -z "$LAN_IP" ]; then
        xray_die "Detected LAN interface '$LAN_IFACE' but no IPv4 address is configured"
    fi

    if ! xray_lan_is_private_ipv4 "$LAN_IP"; then
        xray_log "Warning: detected LAN IPv4 $LAN_IP is not in a private range"
    fi

    xray_log "LAN interface: $LAN_IFACE"
    xray_log "LAN IPv4: $LAN_IP"
}

if [ "${0##*/}" = "lan_detect.sh" ]; then
    script_dir=$(CDPATH= cd -- "$(dirname "$0")" 2>/dev/null && pwd)
    if [ -z "$script_dir" ]; then
        printf 'Error: Unable to determine script directory.\n' >&2
        exit 1
    fi
    bootstrap="$script_dir/bootstrap.sh"
    if [ ! -r "$bootstrap" ]; then
        printf 'Error: Unable to locate XRAY bootstrap helpers.\n' >&2
        exit 1
    fi
    # shellcheck disable=SC1090
    . "$bootstrap"
    xray_bootstrap_run_main "lan_detect.sh" xray_lan_detect_main "$@"
fi
