#!/bin/sh
# XRAY-P2P command dispatcher

SCRIPT_NAME=${0##*/}

if [ -z "${XRAY_SELF_DIR:-}" ]; then
    case "$0" in
        */*)
            XRAY_SELF_DIR=$(CDPATH= cd -- "$(dirname "$0")" 2>/dev/null && pwd)
            export XRAY_SELF_DIR
            ;;
    esac
fi

: "${XRAY_SELF_DIR:=}"
XP2P_SCRIPTS_DIR=${XRAY_SELF_DIR%/}
[ -n "$XP2P_SCRIPTS_DIR" ] || XP2P_SCRIPTS_DIR="."

xp2p_print_available() {
    for file in "$XP2P_SCRIPTS_DIR"/*.sh; do
        [ -f "$file" ] || continue
        base=$(basename "$file")
        case "$base" in
            xp2p.sh|xp2p)
                continue
                ;;
        esac
        pretty=$(printf '%s' "${base%.sh}" | tr '_' ' ')
        printf '  %s\n' "$pretty"
    done
}

xp2p_usage() {
    printf 'Usage: %s <group> [subgroup] [--] [options]\n' "$SCRIPT_NAME"
    printf 'Dispatch helper for XRAY-P2P scripts.\n'
    printf 'Available targets:\n'
    xp2p_print_available
    exit "${1:-0}"
}

xp2p_find_script() {
    max_depth=2
    set -- "$@"
    total=$#
    if [ "$total" -eq 0 ]; then
        return 1
    fi
    if [ "$total" -lt "$max_depth" ]; then
        max_depth=$total
    fi
    depth=$max_depth
    while [ "$depth" -gt 0 ]; do
        candidate=""
        index=1
        for token in "$@"; do
            sanitized=$(printf '%s' "$token" | tr '[:upper:]' '[:lower:]' | tr '-' '_')
            if [ -z "$candidate" ]; then
                candidate="$sanitized"
            else
                candidate="${candidate}_${sanitized}"
            fi
            if [ "$index" -eq "$depth" ]; then
                break
            fi
            index=$((index + 1))
        done
        [ "$candidate" = "xp2p" ] && depth=$((depth - 1)) && continue
        script_path="${XP2P_SCRIPTS_DIR%/}/$candidate.sh"
        if [ -f "$script_path" ]; then
            printf '%s:%s\n' "$depth" "$script_path"
            return 0
        fi
        depth=$((depth - 1))
    done
    return 1
}

main() {
    if [ "$#" -eq 0 ]; then
        xp2p_usage 1
    fi

    case "$1" in
        -h|--help)
            xp2p_usage 0
            ;;
    esac

    dispatch_info=$(xp2p_find_script "$@") || {
        printf 'Error: Unknown target "%s".\n' "$1" >&2
        printf 'Use "%s --help" to list available scripts.\n' "$SCRIPT_NAME" >&2
        exit 1
    }

    consumed=${dispatch_info%%:*}
    script_path=${dispatch_info#*:}

    shift_count=0
    while [ "$shift_count" -lt "$consumed" ]; do
        shift
        shift_count=$((shift_count + 1))
    done

    if [ -x "$script_path" ]; then
        exec "$script_path" "$@"
    else
        exec sh "$script_path" "$@"
    fi
}

main "$@"
