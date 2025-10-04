#!/bin/sh

# Bootstrap helpers shared across XRAY scripts.

if [ "${XRAY_BOOTSTRAP_LOADED:-0}" = "1" ]; then
    return 0 2>/dev/null || true
fi
XRAY_BOOTSTRAP_LOADED=1

xray_bootstrap_detect_self_dir() {
    if [ -n "${XRAY_SELF_DIR:-}" ]; then
        return 0
    fi

    case "$0" in
        */*)
            XRAY_SELF_DIR=$(CDPATH= cd -- "$(dirname "$0")" 2>/dev/null && pwd)
            export XRAY_SELF_DIR
            ;;
    esac
}

xray_bootstrap_load_common() {
    xray_bootstrap_detect_self_dir

    XRAY_COMMON_LIB_PATH_DEFAULT="lib/common.sh"
    XRAY_COMMON_LIB_REMOTE_PATH_DEFAULT="scripts/lib/common.sh"

    lib_local="${XRAY_COMMON_LIB_PATH:-$XRAY_COMMON_LIB_PATH_DEFAULT}"
    lib_remote="${XRAY_COMMON_LIB_REMOTE_PATH:-$XRAY_COMMON_LIB_REMOTE_PATH_DEFAULT}"

    if [ -n "${XRAY_SELF_DIR:-}" ]; then
        candidate="${XRAY_SELF_DIR%/}/$lib_local"
        if [ -r "$candidate" ]; then
            # shellcheck disable=SC1090
            . "$candidate"
            return 0
        fi
    fi

    if [ -r "$lib_local" ]; then
        # shellcheck disable=SC1090
        . "$lib_local"
        return 0
    fi

    base="${XRAY_REPO_BASE_URL:-https://raw.githubusercontent.com/NlightN22/xray-p2p/main}"
    base_trimmed="${base%/}"
    case "$lib_remote" in
        /*)
            lib_url="${base_trimmed}${lib_remote}"
            ;;
        *)
            lib_url="${base_trimmed}/${lib_remote}"
            ;;
    esac

    tmp="$(mktemp 2>/dev/null)" || return 1
    trap 'rm -f "$tmp"' INT TERM HUP

    if command -v xray_download_file >/dev/null 2>&1; then
        if ! xray_download_file "$lib_url" "$tmp" "common library"; then
            rm -f "$tmp"
            trap - INT TERM HUP
            return 1
        fi
    else
        if command -v curl >/dev/null 2>&1 && curl -fsSL "$lib_url" -o "$tmp"; then
            :
        elif command -v wget >/dev/null 2>&1 && wget -q -O "$tmp" "$lib_url"; then
            :
        else
            rm -f "$tmp"
            trap - INT TERM HUP
            return 1
        fi
    fi

    trap - INT TERM HUP
    # shellcheck disable=SC1090
    . "$tmp"
    rm -f "$tmp"
    return 0
}

xray_bootstrap_source_library() {
    local_spec="$1"
    remote_spec="$2"

    resolved=""
    if command -v xray_resolve_local_path >/dev/null 2>&1; then
        resolved="$(xray_resolve_local_path "$local_spec")"
        if [ -r "$resolved" ]; then
            # shellcheck disable=SC1090
            . "$resolved"
            return 0
        fi
    else
        if [ -n "${XRAY_SELF_DIR:-}" ]; then
            candidate="${XRAY_SELF_DIR%/}/$local_spec"
            if [ -r "$candidate" ]; then
                # shellcheck disable=SC1090
                . "$candidate"
                return 0
            fi
        fi

        if [ -r "$local_spec" ]; then
            # shellcheck disable=SC1090
            . "$local_spec"
            return 0
        fi
    fi

    if ! command -v xray_fetch_repo_script >/dev/null 2>&1; then
        return 1
    fi

    tmp="$(xray_fetch_repo_script "$remote_spec")" || return 1
    # shellcheck disable=SC1090
    . "$tmp"
    rm -f "$tmp"
    return 0
}

xray_bootstrap_run_main() {
    script_name="$1"
    main_func="$2"
    shift 2

    if [ "${0##*/}" != "$script_name" ]; then
        return 0
    fi

    if ! command -v "$main_func" >/dev/null 2>&1; then
        printf 'Error: Entry point %s is undefined.\n' "$main_func" >&2
        exit 1
    fi

    if [ -z "${XRAY_COMMON_LIB_PATH:-}" ]; then
        XRAY_COMMON_LIB_PATH="common.sh"
    fi
    if [ -z "${XRAY_COMMON_LIB_REMOTE_PATH:-}" ]; then
        XRAY_COMMON_LIB_REMOTE_PATH="scripts/lib/common.sh"
    fi

    if ! xray_bootstrap_load_common; then
        printf 'Error: Unable to load XRAY common library.\n' >&2
        exit 1
    fi

    "$main_func" "$@"
    exit $?
}
