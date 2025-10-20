#!/bin/sh
# shellcheck shell=ash

SCRIPT_NAME=${0##*/}

# Determine script root for relative lookups
if [ -z "${XRAY_SELF_DIR:-}" ]; then
    case "$0" in
        */*)
            XRAY_SELF_DIR=$(CDPATH= cd -- "$(dirname "$0")" 2>/dev/null && pwd)
            export XRAY_SELF_DIR
            ;;
    esac
fi
: "${XRAY_SELF_DIR:=}"

# Bootstrap common loader (mirrors scripts/server.sh behaviour)
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
    printf 'Error: Unable to initialise XRAY common loader.\n' >&2
    exit 1
fi

if ! load_common_lib; then
    printf 'Error: Unable to load XRAY common library.\n' >&2
    exit 1
fi

if ! xray_common_try_source \
    "${XRAY_SERVER_INSTALL_SELFSIGNED_LIB:-scripts/lib/server_install_cert_selfsigned.sh}" \
    "scripts/lib/server_install_cert_selfsigned.sh" \
    "lib/server_install_cert_selfsigned.sh"; then
    xray_die "Unable to load self-signed certificate helper."
fi

if ! xray_common_try_source \
    "${XRAY_SERVER_INSTALL_APPLY_CERT_LIB:-scripts/lib/server_install_cert_apply.sh}" \
    "scripts/lib/server_install_cert_apply.sh" \
    "lib/server_install_cert_apply.sh"; then
    xray_die "Unable to load certificate apply helper."
fi

usage() {
    cat <<EOF
Usage: $SCRIPT_NAME <command> [options]

Commands:
  selfsigned [--inbounds FILE] [--name CN] [--force]
      Generate or refresh a self-signed certificate for the trojan inbound.

  apply --cert CERT --key KEY [--inbounds FILE]
      Apply existing certificate and key paths to the trojan inbound.

Environment overrides:
  XRAY_REISSUE_CERT   Force regeneration or reuse behaviour for self-signed flow.
  XRAY_SERVER_NAME    Default CN for self-signed certificate generation.
EOF
    exit "${1:-0}"
}

cmd_selfsigned() {
    inbounds="${XRAYP2P_CONFIG_DIR:-/etc/xray-p2p}/inbounds.json"
    server_name="${XRAY_SERVER_NAME:-}"
    force_flag=0

    while [ "$#" -gt 0 ]; do
        case "$1" in
            --inbounds)
                shift
                inbounds="$1"
                ;;
            --name)
                shift
                server_name="$1"
                ;;
            --force)
                force_flag=1
                ;;
            -h|--help)
                usage 0
                ;;
            --)
                shift
                break
                ;;
            -*)
                xray_die "Unknown option for selfsigned: $1"
                ;;
            *)
                xray_die "Unexpected argument for selfsigned: $1"
                ;;
        esac
        shift
    done

    [ -n "$server_name" ] && export XRAY_SERVER_NAME="$server_name"
    if [ "$force_flag" -eq 1 ]; then
        export XRAY_REISSUE_CERT=1
    fi

    server_install_selfsigned_handle "$inbounds"
}

cmd_apply() {
    inbounds="${XRAYP2P_CONFIG_DIR:-/etc/xray-p2p}/inbounds.json"
    cert=""
    key=""

    while [ "$#" -gt 0 ]; do
        case "$1" in
            --cert)
                shift
                cert="$1"
                ;;
            --key)
                shift
                key="$1"
                ;;
            --inbounds)
                shift
                inbounds="$1"
                ;;
            -h|--help)
                usage 0
                ;;
            --)
                shift
                break
                ;;
            -*)
                xray_die "Unknown option for apply: $1"
                ;;
            *)
                xray_die "Unexpected argument for apply: $1"
                ;;
        esac
        shift
    done

    if [ -z "$cert" ] || [ -z "$key" ]; then
        xray_die "Both --cert and --key must be provided."
    fi

    stage_dir="/tmp/scripts/lib"
    cert_paths_cached="$stage_dir/server_cert_paths.sh"
    if [ ! -r "$cert_paths_cached" ]; then
        mkdir -p "$stage_dir" || xray_die "Unable to create directory $stage_dir"
        base_url="https://raw.githubusercontent.com/NlightN22/xray-p2p/main/scripts/lib/server_cert_paths.sh"
        if command -v curl >/dev/null 2>&1; then
            curl -fsSL "$base_url" -o "$cert_paths_cached" || xray_die "Unable to download server_cert_paths helper"
        elif command -v wget >/dev/null 2>&1; then
            wget -q -O "$cert_paths_cached" "$base_url" || xray_die "Unable to download server_cert_paths helper"
        else
            xray_die "Neither curl nor wget available to fetch server_cert_paths helper"
        fi
        chmod +x "$cert_paths_cached" 2>/dev/null || true
    fi

    export XRAY_SERVER_CERT_PATHS_LIB="$cert_paths_cached"
    export XRAY_SELF_DIR="/tmp"
    export XRAY_SCRIPT_ROOT="/tmp"

    server_install_cert_apply_paths "$inbounds" "$cert" "$key"
}

main() {
    command="$1"
    [ -n "$command" ] || usage 1
    shift

    case "$command" in
        selfsigned)
            cmd_selfsigned "$@"
            ;;
        apply)
            cmd_apply "$@"
            ;;
        -h|--help)
            usage 0
            ;;
        *)
            xray_die "Unknown command: $command"
            ;;
    esac
}

main "$@"
