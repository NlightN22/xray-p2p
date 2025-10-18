#!/bin/sh

# Bootstrap helper to load the XRAY common shell library and its companions.

if [ "${XRAY_COMMON_LOADER_LOADED:-0}" = "1" ]; then
    return 0 2>/dev/null || true
fi
XRAY_COMMON_LOADER_LOADED=1

XRAY_COMMON_LOADER_DEFAULT_REMOTE="scripts/lib/common.sh"
XRAY_COMMON_LOADER_DEFAULT_BUNDLE="scripts/lib/bootstrap.sh scripts/lib/common.sh scripts/lib/network_validation.sh scripts/lib/interface_detect.sh scripts/lib/ip_show.sh scripts/lib/lan_detect.sh scripts/lib/network_interfaces.sh scripts/lib/user_list.sh scripts/lib/common_loader.sh scripts/lib/server_install.sh scripts/lib/server_install_port.sh scripts/lib/server_install_cert.sh scripts/lib/server_remove.sh scripts/lib/client_install.sh scripts/lib/client_remove.sh"

xray_common_loader_repo_base() {
    printf '%s' "${XRAY_REPO_BASE_URL:-https://raw.githubusercontent.com/NlightN22/xray-p2p/main}"
}

xray_common_loader_download() {
    url="$1"
    destination="$2"

    if command -v curl >/dev/null 2>&1; then
        if curl -fsSL "$url" -o "$destination"; then
            return 0
        fi
    fi

    if command -v wget >/dev/null 2>&1; then
        if wget -q -O "$destination" "$url"; then
            return 0
        fi
    fi

    return 1
}

xray_common_loader_ensure_cache() {
    if [ -n "${XRAY_LIB_CACHE_DIR:-}" ] && [ -d "$XRAY_LIB_CACHE_DIR" ]; then
        printf '%s' "$XRAY_LIB_CACHE_DIR"
        return 0
    fi

    dir="$(mktemp -d 2>/dev/null)" || return 1
    XRAY_LIB_CACHE_DIR="$dir"
    export XRAY_LIB_CACHE_DIR

    if [ -z "${XRAY_SELF_DIR:-}" ]; then
        XRAY_SELF_DIR="$dir"
        export XRAY_SELF_DIR
    fi

    if [ -z "${XRAY_SCRIPT_ROOT:-}" ]; then
        XRAY_SCRIPT_ROOT="$dir"
        export XRAY_SCRIPT_ROOT
    fi

    printf '%s' "$XRAY_LIB_CACHE_DIR"
}

xray_common_loader_try_source() {
    for path in "$@"; do
        [ -z "$path" ] && continue

        case "$path" in
            /*)
                candidate="$path"
                if [ -r "$candidate" ]; then
                    # shellcheck disable=SC1090
                    . "$candidate"
                    return 0
                fi
                ;;
            *)
                if [ -n "${XRAY_SELF_DIR:-}" ]; then
                    candidate="${XRAY_SELF_DIR%/}/$path"
                    if [ -r "$candidate" ]; then
                        # shellcheck disable=SC1090
                        . "$candidate"
                        return 0
                    fi
                fi

                if [ -n "${XRAY_SCRIPT_ROOT:-}" ]; then
                    candidate="${XRAY_SCRIPT_ROOT%/}/$path"
                    if [ -r "$candidate" ]; then
                        # shellcheck disable=SC1090
                        . "$candidate"
                        return 0
                    fi
                fi

                if [ -r "$path" ]; then
                    # shellcheck disable=SC1090
                    . "$path"
                    return 0
                fi
                ;;
        esac
    done

    return 1
}

xray_common_loader_copy_mirror() {
    source_path="$1"
    mirror_path="$2"

    mirror_dir="$(dirname "$mirror_path")"
    mkdir -p "$mirror_dir" 2>/dev/null || true

    if command -v cp >/dev/null 2>&1; then
        if ! cp "$source_path" "$mirror_path" 2>/dev/null; then
            cat "$source_path" >"$mirror_path" || return 1
        fi
    else
        cat "$source_path" >"$mirror_path" || return 1
    fi

    chmod 0644 "$mirror_path" 2>/dev/null || true
    return 0
}

xray_common_loader_fetch_bundle() {
    remote_common="$1"
    shift

    set -- "$remote_common" "$@"

    base="$(xray_common_loader_repo_base)"
    base="${base%/}"

    cache_dir="$(xray_common_loader_ensure_cache)" || {
        printf 'Error: Unable to prepare cache directory for common loader.\n' >&2
        return 1
    }

    for item in "$@"; do
        [ -z "$item" ] && continue
        rel="${item#./}"
        rel="${rel#/}"
        dest="$cache_dir/$rel"

        if [ -r "$dest" ]; then
            continue
        fi

        url="$base/$rel"
        tmp="$(mktemp 2>/dev/null)" || {
            printf 'Error: Unable to create temporary file while downloading %s.\n' "$rel" >&2
            return 1
        }

        if ! xray_common_loader_download "$url" "$tmp"; then
            printf 'Error: Unable to download %s.\n' "$url" >&2
            rm -f "$tmp"
            return 1
        fi

        dest_dir="$(dirname "$dest")"
        if ! mkdir -p "$dest_dir"; then
            printf 'Error: Unable to create directory %s.\n' "$dest_dir" >&2
            rm -f "$tmp"
            return 1
        fi

        if ! mv "$tmp" "$dest"; then
            printf 'Error: Unable to install %s to %s.\n' "$rel" "$dest" >&2
            rm -f "$tmp"
            return 1
        fi

        chmod 0644 "$dest" 2>/dev/null || true

        case "$rel" in
            scripts/lib/*)
                mirror="$cache_dir/lib/${rel#scripts/lib/}"
                if ! xray_common_loader_copy_mirror "$dest" "$mirror"; then
                    printf 'Error: Unable to mirror %s into cache.\n' "$rel" >&2
                    return 1
                fi
                ;;
        esac
    done

    if [ -z "${XRAY_SCRIPT_ROOT:-}" ]; then
        XRAY_SCRIPT_ROOT="$cache_dir"
        export XRAY_SCRIPT_ROOT
    fi

    if [ -z "${XRAY_SELF_DIR:-}" ]; then
        XRAY_SELF_DIR="$cache_dir"
        export XRAY_SELF_DIR
    fi

    return 0
}

xray_common_loader_load() {
    remote_common="$1"
    shift

    bundle="$remote_common $*"

    set -- "$remote_common"

    if [ -n "${COMMON_LIB_LOCAL_PATH:-}" ]; then
        set -- "$@" "$COMMON_LIB_LOCAL_PATH"
    fi
    if [ -n "${XRAY_COMMON_LIB_PATH:-}" ]; then
        set -- "$@" "$XRAY_COMMON_LIB_PATH"
    fi
    if [ -n "${XRAY_COMMON_LIB_PATH_DEFAULT:-}" ]; then
        set -- "$@" "$XRAY_COMMON_LIB_PATH_DEFAULT"
    fi
    set -- "$@" "lib/common.sh"

    if xray_common_loader_try_source "$@"; then
        return 0
    fi

    if ! xray_common_loader_fetch_bundle "$remote_common" $bundle; then
        return 1
    fi

    set -- "$remote_common"
    if [ -n "${XRAY_LIB_CACHE_DIR:-}" ]; then
        set -- "$@" "$XRAY_LIB_CACHE_DIR/$remote_common" "$XRAY_LIB_CACHE_DIR/lib/common.sh"
    fi
    if [ -n "${COMMON_LIB_LOCAL_PATH:-}" ]; then
        set -- "$@" "$COMMON_LIB_LOCAL_PATH"
    fi
    if [ -n "${XRAY_COMMON_LIB_PATH:-}" ]; then
        set -- "$@" "$XRAY_COMMON_LIB_PATH"
    fi
    if [ -n "${XRAY_COMMON_LIB_PATH_DEFAULT:-}" ]; then
        set -- "$@" "$XRAY_COMMON_LIB_PATH_DEFAULT"
    fi
    set -- "$@" "lib/common.sh"

    if xray_common_loader_try_source "$@"; then
        return 0
    fi

    printf 'Error: Unable to source XRAY common library after download.\n' >&2
    return 1
}

load_common_lib() {
    remote_common="${COMMON_LIB_REMOTE_PATH:-$XRAY_COMMON_LOADER_DEFAULT_REMOTE}"
    bundle_spec="${XRAY_COMMON_LOADER_BUNDLE:-$XRAY_COMMON_LOADER_DEFAULT_BUNDLE}"

    # shellcheck disable=SC2086
    xray_common_loader_load "$remote_common" $bundle_spec
}
