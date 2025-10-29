#!/bin/sh

# Common helper functions shared across XRAY management scripts.

# Prevent double inclusion when sourced multiple times.
if [ "${XRAY_COMMON_LIB_LOADED:-0}" = "1" ]; then
    return 0
fi
XRAY_COMMON_LIB_LOADED=1

xray_common_try_source() {
    for path in "$@"; do
        [ -n "$path" ] || continue
        case "$path" in
            /*)
                if [ -r "$path" ]; then
                    # shellcheck disable=SC1090
                    . "$path"
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

if [ "${XRAY_NETWORK_VALIDATION_LOADED:-0}" != "1" ]; then
    xray_common_try_source \
        "${XRAY_NETWORK_VALIDATION_LIB:-}" \
        "scripts/lib/network_validation.sh" \
        "lib/network_validation.sh" \
        "network_validation.sh" \
        || true
fi

XRAY_DEFAULT_REPO_BASE_URL="https://raw.githubusercontent.com/NlightN22/xray-p2p/main"

xray_log() {
    printf '%s\n' "$*" >&2
}

xray_die() {
    printf 'Error: %s\n' "$*" >&2
    exit 1
}

xray_warn() {
    printf 'Warning: %s\n' "$*" >&2
}

xray_print_table() {
    local tab
    tab=$(printf '\t')
    if [ "${XRAY_TABLE_DISABLE_COLUMN:-0}" != "1" ] && command -v column >/dev/null 2>&1; then
        column -t -s "$tab"
        return
    fi
    awk -v tab="$tab" '
        BEGIN {
            FS = tab
        }
        {
            row_count++
            if (NF > max_fields) {
                max_fields = NF
            }
            for (i = 1; i <= NF; i++) {
                field[row_count, i] = $i
                len = length($i)
                if (len > width[i]) {
                    width[i] = len
                }
            }
        }
        END {
            if (row_count == 0) {
                exit 0
            }
            for (r = 1; r <= row_count; r++) {
                for (c = 1; c <= max_fields; c++) {
                    text = field[r, c]
                    if (c < max_fields) {
                        printf "%-*s", width[c], text
                        if (max_fields > 1) {
                            printf "  "
                        }
                    } else {
                        printf "%s", text
                    }
                }
                printf "\n"
            }
        }
    '
}

xray_require_cmd() {
    cmd="$1"
    if command -v "$cmd" >/dev/null 2>&1; then
        return 0
    fi
    case "$cmd" in
        nft)
            xray_die "Required command 'nft' not found. Install nftables (e.g. opkg update && opkg install nftables)."
            ;;
        jq)
            xray_die "Required command 'jq' not found. Install it before running this script. For OpenWrt run: opkg update && opkg install jq"
            ;;
        uci)
            xray_die "Required command 'uci' not found. Ensure you are running this on OpenWrt."
            ;;
        openssl)
            xray_die "Required command 'openssl' not found. Install it before running this script (e.g. opkg update && opkg install openssl-util)."
            ;;
        *)
            xray_die "Required command '$cmd' not found. Install it before running this script."
            ;;
    esac
}

xray_repo_base_url() {
    printf '%s' "${XRAY_REPO_BASE_URL:-$XRAY_DEFAULT_REPO_BASE_URL}"
}

xray_download_file() {
    download_url="$1"
    download_destination="$2"
    download_description="${3:-resource}"

    if [ -z "$download_url" ] || [ -z "$download_destination" ]; then
        printf 'Error: xray_download_file requires URL and destination path.\n' >&2
        return 1
    fi

    if command -v curl >/dev/null 2>&1; then
        if curl -fsSL "$download_url" -o "$download_destination"; then
            return 0
        fi
    fi

    if command -v wget >/dev/null 2>&1; then
        if wget -q -O "$download_destination" "$download_url"; then
            return 0
        fi
    fi

    printf 'Error: Unable to download %s from %s.\n' "$download_description" "$download_url" >&2
    rm -f "$download_destination"
    return 1
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
    if xray_download_file "$url" "$tmp" "repository script $path"; then
        printf '%s' "$tmp"
        return 0
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

xray_restart_service() {
    service_name="${1:-}"
    service_script="${2:-}"
    skip_var="${3:-}"

    [ -n "$service_name" ] || service_name="${XRAY_SERVICE_NAME:-xray}"
    [ -n "$service_script" ] || service_script="/etc/init.d/$service_name"
    [ -n "$skip_var" ] || skip_var="XRAY_SKIP_RESTART"

    skip_value=""
    if [ -n "$skip_var" ]; then
        skip_value=$(eval "printf '%s' \"\${$skip_var:-}\"")
    fi

    if [ "$skip_value" = "1" ]; then
        xray_log "Skipping ${service_name} restart (${skip_var}=1)."
        return 0
    fi

    if [ ! -x "$service_script" ]; then
        xray_die "Service script not found or not executable: $service_script"
    fi

    local restart_output=""

    xray_log "Restarting ${service_name} service"
    if ! restart_output=$("$service_script" restart 2>&1); then
        if [ -n "$restart_output" ]; then
            printf '%s\n' "$restart_output" >&2
        fi
        xray_die "Failed to restart ${service_name} service."
    fi

    if [ -n "$restart_output" ]; then
        printf '%s\n' "$restart_output" | while IFS= read -r line || [ -n "$line" ]; do
            case "$line" in
                *"Command failed: Not found"*)
                    xray_log "service was not running. Starting..."
                    ;;
                "")
                    ;;
                *)
                    printf '%s\n' "$line"
                    ;;
            esac
        done
    fi

    return 0
}

xray_prompt_yes_no() {
    prompt="$1"
    default_answer="${2:-N}"

    while :; do
        printf '%s' "$prompt" >&2
        if [ -t 0 ]; then
            IFS= read -r answer || answer=""
        elif [ -r /dev/tty ]; then
            IFS= read -r answer </dev/tty || answer=""
        else
            return 2
        fi

        if [ -z "$answer" ]; then
            answer="$default_answer"
        fi

        case "$answer" in
            [Yy])
                return 0
                ;;
            [Nn])
                return 1
                ;;
            *)
                xray_log "Please answer y or n."
                ;;
        esac
    done
}

xray_prompt_line() {
    prompt="$1"
    default_value="${2:-}"
    response=""

    if [ -t 0 ]; then
        printf '%s' "$prompt" >&2
        IFS= read -r response || response=""
    elif [ -r /dev/tty ]; then
        printf '%s' "$prompt" >&2
        IFS= read -r response </dev/tty || response=""
    else
        xray_die "No interactive terminal available. Provide required values via arguments or environment variables."
    fi

    if [ -z "$response" ]; then
        printf '%s' "$default_value"
    else
        printf '%s' "$response"
    fi
}

xray_should_replace_file() {
    target="$1"
    force_var="${2:-XRAY_FORCE_CONFIG}"

    if [ ! -f "$target" ]; then
        return 0
    fi

    force_value=""
    if [ -n "$force_var" ]; then
        force_value=$(eval "printf '%s' \"\${$force_var:-}\"")
    fi

    case "$force_value" in
        1)
            xray_log "Replacing $target (forced by ${force_var}=1)"
            return 0
            ;;
        0)
            xray_log "Keeping existing $target (${force_var}=0)"
            return 1
            ;;
    esac

    if xray_prompt_yes_no "File $target exists. Replace with repository version? [y/N]: " "N"; then
        return 0
    fi

    response=$?
    if [ "$response" -eq 1 ]; then
        xray_log "Keeping existing $target"
        return 1
    fi

    xray_die "No interactive terminal available. Set ${force_var}=1 to overwrite or 0 to keep existing files."
}

xray_seed_file_from_template() {
    destination="$1"
    remote_path="$2"
    local_spec="${3:-$remote_path}"
    force_var="${4:-XRAY_FORCE_CONFIG}"

    if [ -z "$destination" ] || [ -z "$remote_path" ]; then
        xray_die "xray_seed_file_from_template requires destination and remote template paths"
    fi

    dest_dir=$(dirname "$destination")
    if [ ! -d "$dest_dir" ]; then
        mkdir -p "$dest_dir" || xray_die "Unable to create directory $dest_dir"
    fi

    if ! xray_should_replace_file "$destination" "$force_var"; then
        return 0
    fi

    if [ -n "$local_spec" ]; then
        if resolved_path=$(xray_resolve_local_path "$local_spec"); then
            candidate="$resolved_path"
        else
            candidate="$resolved_path"
        fi

        if [ -n "$candidate" ] && [ -r "$candidate" ]; then
            xray_log "Seeding $destination from local template $candidate"
            if ! cp "$candidate" "$destination"; then
                xray_die "Failed to copy template from $candidate"
            fi
            chmod 0644 "$destination" 2>/dev/null || true
            return 0
        fi
    fi

    url=$(xray_repo_build_url "$remote_path")
    tmp=$(mktemp 2>/dev/null) || xray_die "Unable to create temporary file"

    if ! xray_download_file "$url" "$tmp" "template $remote_path"; then
        rm -f "$tmp"
        xray_die "Unable to download template from $url"
    fi

    chmod 0644 "$tmp" 2>/dev/null || true
    if ! mv "$tmp" "$destination"; then
        rm -f "$tmp"
        xray_die "Failed to install template into $destination"
    fi

    xray_log "Downloaded $remote_path to $destination"
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
