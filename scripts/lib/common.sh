#!/bin/sh

# Common helper functions shared across XRAY management scripts.

# Prevent double inclusion when sourced multiple times.
if [ "${XRAY_COMMON_LIB_LOADED:-0}" = "1" ]; then
    return 0
fi
XRAY_COMMON_LIB_LOADED=1

XRAY_DEFAULT_REPO_BASE_URL="https://raw.githubusercontent.com/NlightN22/xray-p2p/main"

xray_log() {
    printf '%s\n' "$*" >&2
}

xray_die() {
    printf 'Error: %s\n' "$*" >&2
    exit 1
}

xray_require_cmd() {
    cmd="$1"
    if command -v "$cmd" >/dev/null 2>&1; then
        return 0
    fi
    case "$cmd" in
        jq)
            xray_die "Required command 'jq' not found. Install it before running this script. For OpenWrt run: opkg update && opkg install jq"
            ;;
        *)
            xray_die "Required command '$cmd' not found. Install it before running this script."
            ;;
    esac
}

# Allow scripts to keep using legacy names without redefining everywhere.
log() {
    xray_log "$@"
}

die() {
    xray_die "$@"
}

require_cmd() {
    xray_require_cmd "$@"
}

xray_repo_base_url() {
    printf '%s' "${XRAY_REPO_BASE_URL:-$XRAY_DEFAULT_REPO_BASE_URL}"
}

xray_repo_build_url() {
    path="$1"
    base="$(xray_repo_base_url)"
    base_trimmed="${base%/}"
    case "$path" in
        /*)
            printf '%s%s' "$base_trimmed" "$path"
            ;;
        *)
            printf '%s/%s' "$base_trimmed" "$path"
            ;;
    esac
}

xray_resolve_local_path() {
    target="$1"
    case "$target" in
        /*)
            printf '%s' "$target"
            return 0
            ;;
    esac

    if [ -n "${XRAY_SELF_DIR:-}" ]; then
        candidate="${XRAY_SELF_DIR%/}/$target"
        if [ -r "$candidate" ]; then
            printf '%s' "$candidate"
            return 0
        fi
    fi

    if [ -n "${XRAY_SCRIPT_ROOT:-}" ]; then
        candidate="${XRAY_SCRIPT_ROOT%/}/$target"
        if [ -r "$candidate" ]; then
            printf '%s' "$candidate"
            return 0
        fi
    fi

    if [ -r "$target" ]; then
        printf '%s' "$target"
        return 0
    fi

    printf '%s' "$target"
    return 1
}

xray_fetch_repo_script() {
    path="$1"
    tmp="$(mktemp 2>/dev/null)" || return 1
    url="$(xray_repo_build_url "$path")"
    if command -v curl >/dev/null 2>&1; then
        if curl -fsSL "$url" -o "$tmp"; then
            printf '%s' "$tmp"
            return 0
        fi
    fi
    if command -v wget >/dev/null 2>&1; then
        if wget -q -O "$tmp" "$url"; then
            printf '%s' "$tmp"
            return 0
        fi
    fi
    rm -f "$tmp"
    return 1
}

xray_run_repo_script() {
    mode="$1"
    shift
    local_spec="$1"
    shift
    remote_spec="$1"
    shift

    resolved="$(xray_resolve_local_path "$local_spec")"
    if [ -r "$resolved" ]; then
        sh "$resolved" "$@"
        return $?
    fi

    tmp="$(xray_fetch_repo_script "$remote_spec")" || {
        if [ "$mode" = "optional" ]; then
            xray_log "Optional script '$remote_spec' unavailable."
            return 1
        fi
        xray_die "Required script '$remote_spec' unavailable."
    }

    sh "$tmp" "$@"
    status=$?
    rm -f "$tmp"
    if [ "$status" -ne 0 ]; then
        if [ "$mode" = "optional" ]; then
            xray_log "Optional script '$remote_spec' exited with status $status."
            return $status
        fi
        xray_die "Script '$remote_spec' exited with status $status."
    fi
    return 0
}

xray_check_repo_access() {
    default_path="$1"
    [ "${XRAY_SKIP_REPO_CHECK:-0}" = "1" ] && return

    check_path="${XRAY_REPO_CHECK_PATH:-$default_path}"
    timeout="${XRAY_REPO_CHECK_TIMEOUT:-5}"

    url="$(xray_repo_build_url "$check_path")"
    last_tool=""

    if command -v curl >/dev/null 2>&1; then
        if curl -fsSL --max-time "$timeout" "$url" >/dev/null 2>&1; then
            return
        fi
        last_tool="curl"
    fi

    if command -v wget >/dev/null 2>&1; then
        if wget -q -T "$timeout" -O /dev/null "$url"; then
            return
        fi
        last_tool="wget"
    fi

    if [ -z "$last_tool" ]; then
        xray_log "Neither curl nor wget is available to verify repository accessibility; skipping check."
        return
    fi

    xray_die "Unable to access repository resource $url (last attempt via $last_tool). Set XRAY_SKIP_REPO_CHECK=1 to bypass."
}
