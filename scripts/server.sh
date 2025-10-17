#!/bin/sh
# Manage XRAY-P2P server lifecycle (OpenWrt)

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

if ! command -v load_common_lib >/dev/null 2>&1; then
    for candidate in \
        "${XRAY_SELF_DIR%/}/scripts/lib/common_loader.sh" \
        "scripts/lib/common_loader.sh" \
        "lib/common_loader.sh"; do
        if [ -n "$candidate" ] && [ -r "$candidate" ]; then
            # shellcheck disable=SC1090
            . "$candidate"
            break
        fi
    done
fi

if ! command -v load_common_lib >/dev/null 2>&1; then
    base="${XRAY_REPO_BASE_URL:-https://raw.githubusercontent.com/NlightN22/xray-p2p/main}"
    loader_url="${base%/}/scripts/lib/common_loader.sh"
    tmp="$(mktemp 2>/dev/null)" || {
        printf 'Error: Unable to create temporary loader script.\n' >&2
        exit 1
    }
    if command -v curl >/dev/null 2>&1 && curl -fsSL "$loader_url" -o "$tmp"; then
        :
    elif command -v wget >/dev/null 2>&1 && wget -q -O "$tmp" "$loader_url"; then
        :
    else
        printf 'Error: Unable to download common loader from %s.\n' "$loader_url" >&2
        rm -f "$tmp"
        exit 1
    fi
    # shellcheck disable=SC1090
    . "$tmp"
    rm -f "$tmp"
fi

if ! command -v load_common_lib >/dev/null 2>&1; then
    printf 'Error: Unable to initialize XRAY common loader.\n' >&2
    exit 1
fi

if ! load_common_lib; then
    printf 'Error: Unable to load XRAY common library.\n' >&2
    exit 1
fi

if ! xray_common_try_source \
    "${XRAY_SERVER_INSTALL_LIB:-scripts/lib/server_install.sh}" \
    "scripts/lib/server_install.sh" \
    "lib/server_install.sh"; then
    xray_die "Unable to load server install library."
fi

if ! xray_common_try_source \
    "${XRAY_SERVER_REMOVE_LIB:-scripts/lib/server_remove.sh}" \
    "scripts/lib/server_remove.sh" \
    "lib/server_remove.sh"; then
    xray_die "Unable to load server remove library."
fi

usage() {
    cat <<EOF
Usage: $SCRIPT_NAME <command> [options]

Commands:
  install [SERVER_NAME] [PORT]   Install XRAY core and configure xray-p2p.
  remove                         Remove xray-p2p service, configuration, and binaries.

Options:
  -h, --help                     Show this help message.
EOF
    exit "${1:-0}"
}

main() {
    [ "$#" -gt 0 ] || usage 1

    subcommand="$1"
    shift

    case "$subcommand" in
        install)
            server_install_run "$@"
            ;;
        remove)
            server_remove_run "$@"
            ;;
        -h|--help)
            usage 0
            ;;
        *)
            xray_log "Unknown command: $subcommand"
            usage 1
            ;;
    esac
}

main "$@"
