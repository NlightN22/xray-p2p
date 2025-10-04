#!/bin/sh

# Library module to detect public IPv4 address using multiple providers.

xray_ip_show_main() {
    ip_opendns=$(dig +short myip.opendns.com @resolver1.opendns.com 2>/dev/null || true)
    ip_cf=$(dig +short whoami.cloudflare @1.1.1.1 2>/dev/null || true)

    ip_ifconfig=$(curl -fsS https://ifconfig.me || echo "")
    ip_checkip=$(curl -fsS https://checkip.amazonaws.com || echo "")

    all=$(printf "%s\n%s\n%s\n%s\n" "$ip_opendns" "$ip_cf" "$ip_ifconfig" "$ip_checkip" \
        | grep -E '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' || true)

    ext_ip=$(printf "%s\n" "$all" | sort | uniq -c | sort -rn | head -n1 | awk '{print $2}')

    [ -z "$ext_ip" ] && ext_ip=$(printf "%s\n%s\n%s\n%s\n" "$ip_opendns" "$ip_cf" "$ip_ifconfig" "$ip_checkip" \
        | grep -E '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' | head -n1)

    printf "%s\n" "$ext_ip"
}

if [ "${0##*/}" = "ip_show.sh" ]; then
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
    xray_bootstrap_run_main "ip_show.sh" xray_ip_show_main "$@"
fi
