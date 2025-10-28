#!/bin/sh
# shellcheck shell=ash
#################

set -eu

SCRIPT_NAME=${0##*/}

print_usage() {
    cat <<EOF
Usage: $SCRIPT_NAME [options] <config-file|->

Extract Trojan connection URLs from an xray inbound configuration.

Options:
  -H, --host HOST   Override destination host when the config does not provide one.
  -h, --help        Show this help message.
EOF
}

die() {
    printf 'Error: %s\n' "$*" >&2
    exit 1
}

warn() {
    printf 'Warning: %s\n' "$*" >&2
}

CONFIG_PATH=""
HOST_OVERRIDE=""

while [ "$#" -gt 0 ]; do
    case "$1" in
        -h|--help)
            print_usage
            exit 0
            ;;
        -H|--host)
            shift || die "Missing value for $1"
            HOST_OVERRIDE=$1
            ;;
        --)
            shift
            break
            ;;
        -*)
            die "Unknown option: $1"
            ;;
        *)
            if [ -n "$CONFIG_PATH" ]; then
                die "Only one config file can be specified"
            fi
            CONFIG_PATH=$1
            ;;
    esac
    shift
done

if [ -z "$CONFIG_PATH" ]; then
    if [ -t 0 ]; then
        warn "No config file provided; reading configuration from stdin. Finish input with Ctrl+D."
    fi
    CONFIG_PATH="-"
fi

if [ "$CONFIG_PATH" != "-" ] && [ ! -r "$CONFIG_PATH" ]; then
    die "Config file not found or unreadable: $CONFIG_PATH"
fi

if ! command -v jq >/dev/null 2>&1; then
    die "jq is required but not installed"
fi

detect_ip_with_ip() {
    if ! command -v ip >/dev/null 2>&1; then
        return 1
    fi
    ip -o -4 addr show scope global 2>/dev/null \
        | awk '!/127\.0\.0\.1/ {split($4, a, "/"); if (a[1] != "") {print a[1]; exit}}'
}

detect_ip_with_ubus() {
    if ! command -v ubus >/dev/null 2>&1 || ! command -v jsonfilter >/dev/null 2>&1; then
        return 1
    fi
    ubus call network.interface.wan status 2>/dev/null \
        | jsonfilter -e '@["ipv4-address"][0].address' 2>/dev/null
}

detect_ip_with_hostname() {
    if ! command -v hostname >/dev/null 2>&1; then
        return 1
    fi
    hostname -I 2>/dev/null | awk '{for (i=1; i<=NF; i++) if ($i ~ /^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$/) {print $i; exit}}'
}

detect_public_ip_simple() {
    if command -v curl >/dev/null 2>&1; then
        curl -fsS https://ifconfig.me 2>/dev/null | awk '/^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$/ {print; exit}'
    elif command -v wget >/dev/null 2>&1; then
        wget -qO- https://ifconfig.me 2>/dev/null | awk '/^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$/ {print; exit}'
    else
        return 1
    fi
}

autodetect_host() {
    detected=""

    detected=$(detect_ip_with_ip || true)
    if [ -z "$detected" ]; then
        detected=$(detect_ip_with_ubus || true)
    fi
    if [ -z "$detected" ]; then
        detected=$(detect_ip_with_hostname || true)
    fi
    if [ -z "$detected" ]; then
        detected=$(detect_public_ip_simple || true)
    fi

    if [ -n "$detected" ]; then
        DEFAULT_HOST="$detected"
        return 0
    fi

    return 1
}

DEFAULT_HOST=${HOST_OVERRIDE:-${XRAY_TROJAN_HOST:-}}

if [ -z "$DEFAULT_HOST" ]; then
    if autodetect_host; then
        warn "Host not provided; using auto-detected value $DEFAULT_HOST"
    fi
fi

if [ -z "$DEFAULT_HOST" ]; then
    warn "Unable to infer host automatically. Pass --host or set XRAY_TROJAN_HOST for fully qualified URLs."
fi

jq \
    --arg default_host "$DEFAULT_HOST" \
    '
    (.inbounds // [])[]
    | select((.protocol // "") == "trojan")
    | {
        port: (.port // null),
        listen: (.listen // ""),
        stream: (.streamSettings // {}),
        clients: (.settings.clients // [])
      } as $inb
    | ($inb.stream.network // "tcp") as $network
    | (($inb.stream.security // "tls") | ascii_downcase) as $security
    | ($inb.stream.tlsSettings // {}) as $tls
    | ($tls.serverName // "") as $sni
    | ($inb.listen // "") as $listen
    | (
        if $sni != "" then $sni
        elif $listen == "0.0.0.0" or $listen == "::" then ""
        elif $listen != "" then $listen
        else ""
        end
      ) as $host
    | $inb.clients[]
    | {
        password: (.password // ""),
        label: (.email // ""),
        host: (if $host != "" then $host else $default_host end),
        port: (
            ($inb.port // null)
            | if type == "number" then .
              elif type == "string" then (try tonumber catch null)
              else null end
          ),
        network: ($network | ascii_downcase),
        security: $security,
        allow: (
            ($tls.allowInsecure // false) |
            if type == "string" then
                (ascii_downcase | (if . == "true" or . == "1" or . == "yes" then "true" else "false" end))
            elif . == true then "true"
            else "false"
            end
        ),
        sni: $sni
      }
    | select(.password != "" and .host != "" and (.port | type == "number"))
    | "trojan://" + .password + "@" + .host + ":" + (.port|tostring)
      + "?security=" + .security
      + "&type=" + .network
      + "&allowInsecure=" + .allow
      + (if .sni != "" then "&sni=" + (.sni|@uri) else "" end)
      + (if .label != "" then "#" + (.label|@uri) else "" end)
    ' \
    "$CONFIG_PATH" |
    awk 'BEGIN{found=0} {print; found=1} END{if (found==0) print "Warning: No trojan users found." >"/dev/stderr"}'
