#!/bin/sh
# shellcheck shell=ash

[ "${SERVER_INSTALL_PORT_LIB_LOADED:-0}" = "1" ] && return 0
SERVER_INSTALL_PORT_LIB_LOADED=1

server_install_determine_port() {
    port_arg="$1"

    if [ -n "$port_arg" ]; then
        XRAY_PORT="$port_arg"
    elif [ -n "$XRAY_PORT" ]; then
        xray_log "Using XRAY_PORT=$XRAY_PORT from environment"
    else
        printf "Enter external port for XRAY [%s]: " "$DEFAULT_XRAY_PORT" >&2
        if [ -t 0 ]; then
            IFS= read -r XRAY_PORT
        elif [ -r /dev/tty ]; then
            IFS= read -r XRAY_PORT </dev/tty
        else
            xray_die "No interactive terminal available. Provide port as argument or set XRAY_PORT."
        fi
    fi

    [ -n "$XRAY_PORT" ] || XRAY_PORT="$DEFAULT_XRAY_PORT"
    server_install_validate_port "$XRAY_PORT"
}

server_install_validate_port() {
    port_value="$1"
    echo "$port_value" | grep -Eq "^[0-9]+$" || xray_die "Port must be numeric"
    [ "$port_value" -gt 0 ] && [ "$port_value" -le 65535 ] || xray_die "Port must be between 1 and 65535"
}

server_install_update_inbound() {
    inbound_path="$1"
    port_value="$2"
    tmp_inbound=$(mktemp) || xray_die "Unable to create temporary file for inbound update"
    if ! jq --argjson port "$port_value" '
        .inbounds |= (map(
            if (.protocol // "") == "trojan" then .port = $port else . end
        ))
    ' "$inbound_path" >"$tmp_inbound"; then
        rm -f "$tmp_inbound"
        xray_die "Failed to update inbound port"
    fi
    mv "$tmp_inbound" "$inbound_path"
    if ! jq -e --argjson port "$port_value" 'any(.inbounds[]?; (.protocol // "") == "trojan" and (.port // 0) == $port)' "$inbound_path" >/dev/null 2>&1; then
        xray_die "Failed to update port in $inbound_path"
    fi
}

server_install_detect_port_tool() {
    if command -v ss >/dev/null 2>&1; then
        printf 'ss'
        return 0
    fi
    if command -v netstat >/dev/null 2>&1; then
        printf 'netstat'
        return 0
    fi
    return 1
}

server_install_port_in_use() {
    tool="$1"
    port="$2"
    case "$tool" in
        ss)
            ss -ltn 2>/dev/null | awk -v port="$port" '
                NR <= 1 { next }
                {
                    c = split($4, addr, ":")
                    if (c > 0 && addr[c] == port) exit 0
                }
                END { exit 1 }
            '
            return $?
            ;;
        netstat)
            netstat -tln 2>/dev/null | awk -v port="$port" '
                NR <= 2 { next }
                {
                    c = split($4, addr, ":")
                    if (c > 0 && addr[c] == port) exit 0
                }
                END { exit 1 }
            '
            return $?
            ;;
        *)
            return 1
            ;;
    esac
}

server_install_collect_ports() {
    inbound_path="$1"
    ports=""

    inbound_ports=$(jq -r '
        [.inbounds[]? | .port? | select((type == "number") or (type == "string" and test("^[0-9]+$"))) | tonumber]
        | unique
        | .[]
    ' "$inbound_path" 2>/dev/null)
    if [ -n "$inbound_ports" ]; then
        for p in $inbound_ports; do
            case " $ports " in
                *" $p "*)
                    ;;
                *)
                    ports="${ports:+$ports }$p"
                    ;;
            esac
        done
    fi

    api_tags=$(jq -r '
        .api // empty
        | (if type == "object" then [.tag?] else [] end)
        | flatten
        | unique
        | .[]
    ' "$inbound_path" 2>/dev/null)

    if [ -n "$api_tags" ]; then
        for tag in $api_tags; do
            port=$(jq -r --arg tag "$tag" '
                [.inbounds[]? | select((.tag // "") == $tag) | .port? | select((type == "number") or (type == "string" and test("^[0-9]+$"))) | tonumber]
                | unique
                | first
            ' "$inbound_path" 2>/dev/null)
            if [ -n "$port" ]; then
                case " $ports " in
                    *" $port "*)
                        ;;
                    *)
                        ports="${ports:+$ports }$port"
                        ;;
                esac
            fi
        done
    fi

    printf '%s' "$ports"
}

server_install_preflight_ports() {
    inbound_path="$1"
    [ "${XRAY_SKIP_PORT_CHECK:-0}" = "1" ] && return

    required_ports=$(server_install_collect_ports "$inbound_path")
    [ -n "$required_ports" ] || required_ports="$XRAY_PORT"

    tool=""
    if ! tool=$(server_install_detect_port_tool); then
        xray_log "Skipping preflight port check because neither 'ss' nor 'netstat' is available."
        return
    fi

    collisions=""
    for port in $required_ports; do
        if server_install_port_in_use "$tool" "$port"; then
            collisions="${collisions:+$collisions }$port"
        fi
    done
    [ -z "$collisions" ] || xray_die "Required port(s) already in use: $collisions. Free these ports or set XRAY_SKIP_PORT_CHECK=1 to override."
}
