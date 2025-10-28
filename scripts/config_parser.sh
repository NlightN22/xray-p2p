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
    print_usage >&2
    exit 1
fi

if [ "$CONFIG_PATH" != "-" ] && [ ! -r "$CONFIG_PATH" ]; then
    die "Config file not found or unreadable: $CONFIG_PATH"
fi

if ! command -v jq >/dev/null 2>&1; then
    die "jq is required but not installed"
fi

DEFAULT_HOST=${HOST_OVERRIDE:-${XRAY_TROJAN_HOST:-}}

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
    "$CONFIG_PATH"
