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
  install [SERVER_NAME] [PORT] [--cert CERT_FILE --key KEY_FILE] [--force]
                                 Install XRAY core and configure xray-p2p.
                                 Optional --cert/--key attempt to set TLS
                                 certificate/key paths; on failure, continue
                                 and generate a self-signed certificate in the
                                 default location. --force overwrites existing
                                 files without prompting.
  remove [--purge-core]          Remove xray-p2p service/config; optional purge removes xray-core package.

Options:
  -h, --help                     Show this help message.
  remove --purge-core            Also uninstall xray-core package during cleanup.
EOF
    exit "${1:-0}"
}

main() {
    [ "$#" -gt 0 ] || usage 1

    subcommand="$1"
    shift

    case "$subcommand" in
        install)
            cert_path=""
            key_path=""
            force_mode=0
            # collect remaining args to pass into server_install_run
            pass_args=""
            while [ "$#" -gt 0 ]; do
                case "$1" in
                    --cert)
                        shift
                        cert_path="$1"
                        ;;
                    --key)
                        shift
                        key_path="$1"
                        ;;
                    --force)
                        force_mode=1
                        ;;
                    -h|--help)
                        # let install lib print detailed help; pass through
                        pass_args="$pass_args $1"
                        ;;
                    --)
                        shift
                        # append the rest verbatim and break
                        while [ "$#" -gt 0 ]; do
                            pass_args="$pass_args $1"
                            shift
                        done
                        break
                        ;;
                    *)
                        pass_args="$pass_args $1"
                        ;;
                esac
                shift || true
            done

            if [ -n "$cert_path" ] || [ -n "$key_path" ]; then
                if [ -z "$cert_path" ] || [ -z "$key_path" ]; then
                    xray_warn "Both --cert and --key must be provided; ignoring provided path(s)."
                    unset XRAY_CERT_FILE XRAY_KEY_FILE
                else
                    export XRAY_CERT_FILE="$cert_path"
                    export XRAY_KEY_FILE="$key_path"
                fi
            fi

            if [ "${force_mode:-0}" -eq 1 ]; then
                export XRAY_FORCE_CONFIG=1
            fi

            # shellcheck disable=SC2086
            server_install_run $pass_args
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
