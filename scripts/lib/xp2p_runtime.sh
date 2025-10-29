#!/bin/sh
# XRAY-P2P runtime support (sourced by xp2p.sh)

if [ "${XP2P_RUNTIME_LOADED:-0}" = "1" ]; then
    return 0 2>/dev/null || true
fi
XP2P_RUNTIME_LOADED=1

: "${XP2P_REMOTE_BASE:=https://raw.githubusercontent.com/NlightN22/xray-p2p/main}"
XP2P_REMOTE_BASE=${XP2P_REMOTE_BASE%/}

: "${XP2P_SCRIPTS_DIR:=.}"

XP2P_CORE_FILES="client.sh client_reverse.sh client_user.sh server.sh server_reverse.sh server_user.sh server_cert.sh redirect.sh dns_forward.sh config_parser.sh xsetup.sh"
XP2P_LIB_FILES="lib/bootstrap.sh lib/client_connection.sh lib/client_install.sh lib/client_remove.sh lib/client_reverse_inputs.sh lib/client_reverse_routing.sh lib/client_reverse_store.sh lib/common.sh lib/common_loader.sh lib/dns_forward_core.sh lib/dns_forward_store.sh lib/interface_detect.sh lib/ip_show.sh lib/lan_detect.sh lib/network_interfaces.sh lib/network_validation.sh lib/redirect.sh lib/reverse_common.sh lib/server_cert_paths.sh lib/server_install.sh lib/server_install_cert_apply.sh lib/server_install_cert_selfsigned.sh lib/server_install_port.sh lib/server_remove.sh lib/server_reverse_inputs.sh lib/server_reverse_routing.sh lib/server_reverse_store.sh lib/server_user_common.sh lib/server_user_issue.sh lib/server_user_remove.sh lib/user_list.sh lib/xp2p_runtime.sh"

xp2p_is_openwrt() {
    [ "${XP2P_FORCE_OPENWRT:-}" = "1" ] && return 0
    [ -r /etc/openwrt_release ] && return 0
    [ -r /etc/openwrt_version ] && return 0
    [ -r /etc/os-release ] && grep -i 'openwrt' /etc/os-release >/dev/null 2>&1 && return 0
    (command -v uci >/dev/null 2>&1 || command -v ubus >/dev/null 2>&1) && return 0
    command -v opkg >/dev/null 2>&1 && return 0
    return 1
}

xp2p_candidate_dirs() {
    if xp2p_is_openwrt; then
        [ "$(id -u 2>/dev/null)" = "0" ] && printf '%s\n' "/usr/libexec/xp2p"
        if [ -n "${HOME:-}" ] && [ "$HOME" != "/" ]; then
            printf '%s\n' "${HOME%/}/xray-p2p"
        fi
    fi
    printf '%s\n' "xray-p2p"
}

xp2p_select_install_dir() {
    requested="$1"
    if [ -n "$requested" ]; then
        printf '%s\n' "$requested"
        return 0
    fi
    for candidate in $(xp2p_candidate_dirs); do
        [ -n "$candidate" ] || continue
        if [ -d "$candidate" ]; then
            printf '%s\n' "$candidate"
            return 0
        fi
        if mkdir -p "$candidate" 2>/dev/null; then
            printf '%s\n' "$candidate"
            return 0
        fi
    done
    printf '%s\n' "xray-p2p"
}

xp2p_download_file() {
    rel="$1"
    base_dir="$2"
    mode="$3"

    [ -n "$base_dir" ] || base_dir="$XP2P_SCRIPTS_DIR"
    dest="${base_dir%/}/$rel"
    dest_dir=$(dirname "$dest")

    if [ ! -d "$dest_dir" ] && ! mkdir -p "$dest_dir"; then
        [ "$mode" = "quiet" ] || printf 'Error: Unable to create directory %s.\n' "$dest_dir" >&2
        return 1
    fi

    url="$XP2P_REMOTE_BASE/scripts/$rel"
    tmp="$(mktemp 2>/dev/null)" || {
        [ "$mode" = "quiet" ] || printf 'Error: Unable to create temporary file while fetching %s.\n' "$rel" >&2
        return 1
    }

    fetch_status=1
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url" -o "$tmp" && fetch_status=0
    elif command -v wget >/dev/null 2>&1; then
        wget -q -O "$tmp" "$url" && fetch_status=0
    else
        fetch_status=2
    fi

    if [ "$fetch_status" -ne 0 ]; then
        rm -f "$tmp"
        if [ "$fetch_status" -eq 2 ]; then
            [ "$mode" = "quiet" ] || printf 'Error: Neither curl nor wget is available to download %s.\n' "$rel" >&2
        else
            [ "$mode" = "quiet" ] || printf 'Error: Unable to download %s.\n' "$url" >&2
        fi
        return 1
    fi

    if ! mv "$tmp" "$dest"; then
        if ! cat "$tmp" >"$dest"; then
            rm -f "$tmp"
            [ "$mode" = "quiet" ] || printf 'Error: Unable to install %s.\n' "$dest" >&2
            return 1
        fi
        rm -f "$tmp"
    fi

    case "$rel" in
        lib/*) chmod 0644 "$dest" 2>/dev/null || true ;;
        *) chmod 0755 "$dest" 2>/dev/null || true ;;
    esac
}

xp2p_ensure_rel_file() {
    rel="$1"
    mode="$2"
    dest="${XP2P_SCRIPTS_DIR%/}/$rel"
    [ -f "$dest" ] && return 0
    [ "$mode" = "quiet" ] || printf 'Fetching %s...\n' "$rel"
    xp2p_download_file "$rel" "$XP2P_SCRIPTS_DIR" "$mode"
}

xp2p_auto_bootstrap() {
    [ "${XP2P_AUTO_BOOTSTRAP_DONE:-0}" = "1" ] && return 0
    for file in $XP2P_CORE_FILES $XP2P_LIB_FILES; do
        xp2p_ensure_rel_file "$file" || return 1
    done
    XP2P_AUTO_BOOTSTRAP_DONE=1
}

xp2p_print_available_dir() {
    dir="$1"
    [ -n "$dir" ] || return 0
    for file in "$dir"/*.sh; do
        [ -f "$file" ] || continue
        base=$(basename "$file")
        case "$base" in
            xp2p.sh|xp2p) continue ;;
        esac
        printf '  %s\n' "$(printf '%s' "${base%.sh}" | tr '_' ' ')"
    done
}

xp2p_print_available() {
    xp2p_print_available_dir "$XP2P_SCRIPTS_DIR"
}

xp2p_post_install_summary() {
    dir="$1"
    printf 'Run: %s/xp2p.sh <command> ...\n' "$dir"
    printf '\nAvailable targets:\n'
    xp2p_print_available_dir "$dir"
    printf '\nNext steps:\n'
    printf '  sh %s/xp2p.sh --help\n' "$dir"
    printf '  sh %s/xp2p.sh <group> ...\n' "$dir"
    if command -v xp2p >/dev/null 2>&1; then
        printf '  xp2p <group> ...\n'
    fi
}

xp2p_install_usage() {
    printf '%s\n' "Usage: xp2p install [--dir PATH] [--force]
Download the XRAY-P2P script bundle into PATH (default: ./xray-p2p or /usr/libexec/xp2p on OpenWrt).
Creates PATH/scripts with xp2p.sh and helper scripts.
Options:
  --dir PATH    Target directory for installation.
  --force       Overwrite existing files inside PATH.
  -h, --help    Show this message."
}

xp2p_cmd_install() {
    target_dir=""
    force_mode=0

    while [ "$#" -gt 0 ]; do
        case "$1" in
            --dir)
                shift
                [ "$#" -gt 0 ] || {
                    printf 'Error: --dir requires a path.\n' >&2
                    return 1
                }
                target_dir="$1"
                ;;
            --force) force_mode=1 ;;
            -h|--help)
                xp2p_install_usage
                return 0
                ;;
            *)
                if [ -n "$target_dir" ]; then
                    printf 'Error: Unexpected argument "%s".\n' "$1" >&2
                    return 1
                fi
                target_dir="$1"
                ;;
        esac
        shift
    done

    target_dir="$(xp2p_select_install_dir "$target_dir")"

    if [ -e "$target_dir" ] && [ ! -d "$target_dir" ]; then
        printf 'Error: %s exists and is not a directory.\n' "$target_dir" >&2
        return 1
    fi

    scripts_dir="${target_dir%/}/scripts"

    if [ -d "$target_dir" ] && [ "$force_mode" -ne 1 ] && [ "$(ls -A "$target_dir" 2>/dev/null)" ]; then
        if [ -f "$scripts_dir/xp2p.sh" ]; then
            printf 'XRAY-P2P scripts already installed in %s.\n' "$scripts_dir"
            if xp2p_is_openwrt && [ "$(id -u 2>/dev/null)" = "0" ] && ! command -v xp2p >/dev/null 2>&1; then
                case "$scripts_dir" in
                    /usr/libexec/xp2p/scripts)
                        if ln -sf "$scripts_dir/xp2p.sh" /usr/bin/xp2p 2>/dev/null; then
                            printf 'Symlink created: %s -> %s\n' "/usr/bin/xp2p" "$scripts_dir/xp2p.sh"
                        else
                            printf 'Warning: Unable to create symlink %s.\n' "/usr/bin/xp2p" >&2
                        fi
                        ;;
                esac
            fi
            xp2p_post_install_summary "$scripts_dir"
            return 0
        fi
        printf 'Error: %s is not empty. Use --force to overwrite.\n' "$target_dir" >&2
        return 1
    fi

    printf 'Preparing XRAY-P2P installation in %s...\n' "$scripts_dir"
    if ! mkdir -p "$scripts_dir"; then
        printf 'Error: Unable to create %s.\n' "$scripts_dir" >&2
        return 1
    fi

    bundle_list="$XP2P_CORE_FILES $XP2P_LIB_FILES xp2p.sh"

    for file in $bundle_list; do
        printf 'Downloading %s...\n' "$file"
        if ! xp2p_download_file "$file" "$scripts_dir"; then
            printf 'Error: Installation failed while downloading %s.\n' "$file" >&2
            return 1
        fi
    done

    printf 'XRAY-P2P scripts installed into %s.\n' "$scripts_dir"

    if xp2p_is_openwrt && [ "$(id -u 2>/dev/null)" = "0" ]; then
        case "$scripts_dir" in
            /usr/libexec/xp2p/scripts)
                link_target="/usr/bin/xp2p"
                if ln -sf "$scripts_dir/xp2p.sh" "$link_target" 2>/dev/null; then
                    printf 'Symlink created: %s -> %s\n' "$link_target" "$scripts_dir/xp2p.sh"
                else
                    printf 'Warning: Unable to create symlink %s.\n' "$link_target" >&2
                fi
                ;;
        esac
    fi

    xp2p_post_install_summary "$scripts_dir"
}

xp2p_find_script() {
    max_depth=2
    set -- "$@"
    total=$#
    [ "$total" -gt 0 ] || return 1
    [ "$total" -lt "$max_depth" ] && max_depth=$total

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
            [ "$index" -eq "$depth" ] && break
            index=$((index + 1))
        done
        [ "$candidate" = "xp2p" ] && depth=$((depth - 1)) && continue
        rel="$candidate.sh"
        if xp2p_ensure_rel_file "$rel" "quiet"; then
            script_path="${XP2P_SCRIPTS_DIR%/}/$rel"
            if [ -f "$script_path" ]; then
                printf '%s:%s\n' "$depth" "$script_path"
                return 0
            fi
        fi
        depth=$((depth - 1))
    done
    return 1
}

xp2p_usage() {
    printf 'Usage: %s <group> [subgroup] [--] [options]\n' "$SCRIPT_NAME"
    printf '       %s install [--dir PATH] [--force]\n' "$SCRIPT_NAME"
    printf 'Dispatch helper for XRAY-P2P scripts. Missing scripts are downloaded automatically.\n\n'
    printf 'Available targets:\n'
    xp2p_print_available
    exit "${1:-0}"
}

xp2p_runtime_main() {
    if [ "$#" -eq 0 ] && [ "${XP2P_PIPE_AUTO_INSTALL:-1}" != "0" ]; then
        case "$SCRIPT_NAME" in
            sh|dash|bash)
                if [ ! -t 0 ]; then
                    xp2p_cmd_install
                    exit $?
                fi
                ;;
        esac
    fi

    if [ "$#" -eq 0 ]; then
        xp2p_auto_bootstrap || exit 1
        xp2p_usage 1
    fi

    case "$1" in
        -h|--help)
            xp2p_auto_bootstrap || exit 1
            xp2p_usage 0
            ;;
        install)
            shift
            xp2p_cmd_install "$@"
            exit $?
            ;;
    esac

    xp2p_auto_bootstrap || {
        printf 'Error: Unable to prepare XRAY-P2P scripts.\n' >&2
        exit 1
    }

    dispatch_info=$(xp2p_find_script "$@") || {
        printf 'Error: Unknown target "%s".\n' "$1" >&2
        printf 'Use "%s --help" to list available scripts or "%s install".\n' "$SCRIPT_NAME" "$SCRIPT_NAME" >&2
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
