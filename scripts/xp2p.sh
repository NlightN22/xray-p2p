#!/bin/sh
# XRAY-P2P bootstrap

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

XP2P_REMOTE_BASE=${XRAY_REPO_BASE_URL:-https://raw.githubusercontent.com/NlightN22/xray-p2p/main}
XP2P_REMOTE_BASE=${XP2P_REMOTE_BASE%/}

xp2p_try_source() {
    for candidate in "$@"; do
        [ -r "$candidate" ] || continue
        candidate_dir=$(dirname "$candidate")
        scripts_candidate=""
        case "$candidate" in
            /*)
                case "$candidate_dir" in
                    */lib)
                        scripts_candidate=$(dirname "$candidate_dir")
                        ;;
                esac
                ;;
            *)
                if [ -n "$candidate_dir" ] && [ "$candidate_dir" != "." ]; then
                    candidate_dir_abs=$(CDPATH= cd -- "$candidate_dir" 2>/dev/null && pwd)
                    if [ -n "$candidate_dir_abs" ]; then
                        case "$candidate_dir_abs" in
                            */lib)
                                scripts_candidate=$(dirname "$candidate_dir_abs")
                                ;;
                        esac
                    fi
                fi
                ;;
        esac
        if [ -n "$scripts_candidate" ]; then
            XP2P_SCRIPTS_DIR="$scripts_candidate"
            export XP2P_SCRIPTS_DIR
        fi
        # shellcheck disable=SC1090
        . "$candidate"
        return 0
    done
    return 1
}

xp2p_fetch_runtime() {
    if xp2p_try_source \
        "${XP2P_SCRIPTS_DIR%/}/lib/xp2p_runtime.sh" \
        "${XP2P_SCRIPTS_DIR%/}/../lib/xp2p_runtime.sh" \
        "/usr/libexec/xp2p/scripts/lib/xp2p_runtime.sh" \
        "${HOME:-}/xray-p2p/scripts/lib/xp2p_runtime.sh" \
        "lib/xp2p_runtime.sh"; then
        return 0
    fi

    url="$XP2P_REMOTE_BASE/scripts/lib/xp2p_runtime.sh"
    tmp="$(mktemp 2>/dev/null)" || {
        printf 'Error: Unable to create temporary runtime file.\n' >&2
        return 1
    }

    if command -v curl >/dev/null 2>&1; then
        if ! curl -fsSL "$url" -o "$tmp"; then
            printf 'Error: Unable to download runtime from %s.\n' "$url" >&2
            rm -f "$tmp"
            return 1
        fi
    elif command -v wget >/dev/null 2>&1; then
        if ! wget -q -O "$tmp" "$url"; then
            printf 'Error: Unable to download runtime from %s.\n' "$url" >&2
            rm -f "$tmp"
            return 1
        fi
    else
        printf 'Error: Neither curl nor wget is available to download runtime.\n' >&2
        rm -f "$tmp"
        return 1
    fi

    # shellcheck disable=SC1090
    . "$tmp"
    rm -f "$tmp"
}

xp2p_fetch_runtime || exit 1

xp2p_runtime_main "$@"
