#!/bin/sh

# Library module to present active network interfaces and addresses.

xray_network_interfaces_show_with_ip() {
    ip -o addr show up 2>/dev/null | awk '
        {
            iface=$2
            sub(/@.*/, "", iface)
            if (iface == "lo") next
            family=$3
            addr=$4
            scope=""
            for (i = 5; i <= NF; i++) {
                if ($i == "scope" && (i + 1) <= NF) {
                    scope=$(i + 1)
                    break
                }
            }
            if (scope == "host") next
            split(addr, parts, "/")
            printf "  %s (%s) %s", iface, family, parts[1]
            if (scope != "" && scope != "global") {
                printf " [%s]", scope
            }
            printf "\n"
        }
    '
}

xray_network_interfaces_show_with_ifconfig() {
    ifconfig 2>/dev/null | awk '
        /^[[:space:]]*$/ { next }
        /^[[:alnum:]]/ {
            iface=$1
            sub("[:]+$", "", iface)
            next
        }
        /inet[[:space:]]/ {
            if (iface != "lo") printf "  %s (inet) %s\n", iface, $2
        }
        /inet6[[:space:]]/ {
            if (iface != "lo") printf "  %s (inet6) %s\n", iface, $2
        }
    '
}

xray_network_interfaces_main() {
    if command -v ip >/dev/null 2>&1; then
        addresses="$(xray_network_interfaces_show_with_ip)"
        if [ -n "$addresses" ]; then
            printf 'Detected network interfaces and addresses:\n'
            printf '%s\n' "$addresses"
            return 0
        fi
    fi

    if command -v ifconfig >/dev/null 2>&1; then
        addresses="$(xray_network_interfaces_show_with_ifconfig)"
        if [ -n "$addresses" ]; then
            printf 'Detected network interfaces and addresses:\n'
            printf '%s\n' "$addresses"
            return 0
        fi
    fi

    printf 'No active non-loopback interface addresses detected.\n' >&2
    return 1
}

if [ "${0##*/}" = "network_interfaces.sh" ]; then
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
    xray_bootstrap_run_main "network_interfaces.sh" xray_network_interfaces_main "$@"
fi
