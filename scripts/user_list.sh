#!/bin/sh
set -eu

if [ -z "${XRAY_SELF_DIR:-}" ]; then
    case "$0" in
        */*)
            XRAY_SELF_DIR=$(CDPATH= cd -- "$(dirname "$0")" 2>/dev/null && pwd)
            export XRAY_SELF_DIR
            ;;
    esac
fi

umask 077

XRAY_COMMON_LIB_PATH_DEFAULT="lib/common.sh"
XRAY_COMMON_LIB_REMOTE_PATH_DEFAULT="scripts/lib/common.sh"

load_common_lib() {
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
    if command -v curl >/dev/null 2>&1 && curl -fsSL "$lib_url" -o "$tmp"; then
        # shellcheck disable=SC1090
        . "$tmp"
        rm -f "$tmp"
        return 0
    fi
    if command -v wget >/dev/null 2>&1 && wget -q -O "$tmp" "$lib_url"; then
        # shellcheck disable=SC1090
        . "$tmp"
        rm -f "$tmp"
        return 0
    fi
    rm -f "$tmp"
    return 1
}

if ! load_common_lib; then
    printf 'Error: Unable to load XRAY common library.\n' >&2
    exit 1
fi

CONFIG_DIR="${XRAY_CONFIG_DIR:-/etc/xray}"
INBOUNDS_FILE="${XRAY_INBOUNDS_FILE:-$CONFIG_DIR/inbounds.json}"
CLIENTS_DIR="${XRAY_CLIENTS_DIR:-$CONFIG_DIR/config}"
CLIENTS_FILE="${XRAY_CLIENTS_FILE:-$CLIENTS_DIR/clients.json}"

require_cmd jq

xray_check_repo_access 'scripts/user_list.sh'

if [ ! -f "$CLIENTS_FILE" ]; then
    die "Clients registry not found: $CLIENTS_FILE"
fi

if [ ! -f "$INBOUNDS_FILE" ]; then
    die "Inbound configuration not found: $INBOUNDS_FILE"
fi

if ! jq empty "$CLIENTS_FILE" >/dev/null 2>&1; then
    die "Clients registry $CLIENTS_FILE contains invalid JSON."
fi

if ! jq empty "$INBOUNDS_FILE" >/dev/null 2>&1; then
    die "Inbound configuration $INBOUNDS_FILE contains invalid JSON."
fi

CLIENTS_TMP=$(mktemp)
INBOUND_TMP=$(mktemp)
MISMATCH_TMP=$(mktemp)
ACTIVE_TMP=$(mktemp)

cleanup() {
    rm -f "$CLIENTS_TMP" "$INBOUND_TMP" "$MISMATCH_TMP" "$ACTIVE_TMP"
}
trap cleanup EXIT INT TERM

jq -r '
    map({
        email: (.email // ""),
        password: (.password // ""),
        status: (.status // "")
    })
    | map(select(.email != ""))
    | map([.email, .password, .status] | @tsv)
    | .[]
' "$CLIENTS_FILE" > "$CLIENTS_TMP"

jq -r '
    ( .inbounds // [] )
    | map(select((.protocol // "") == "trojan") | (.settings.clients // []))
    | add // []
    | map({
        email: (.email // ""),
        password: (.password // "")
    })
    | map(select(.email != ""))
    | map([.email, .password] | @tsv)
    | .[]
' "$INBOUNDS_FILE" > "$INBOUND_TMP"

awk_status=0

if ! awk -F '\t' \
    -v mismatch_file="$MISMATCH_TMP" \
    -v active_file="$ACTIVE_TMP" \
    'NR==FNR {
        inbound[$1] = $2
        next
    }
    {
        email = $1
        pass = $2
        status = $3
        if (!(email in inbound)) {
            printf "missing_in_inbounds\t%s\n", email >> mismatch_file
            mismatch = 1
            next
        }
        if (inbound[email] != pass) {
            printf "password_mismatch\t%s\t%s\t%s\n", email, pass, inbound[email] >> mismatch_file
            mismatch = 1
            delete inbound[email]
            next
        }
        status_lc = tolower(status)
        if (status_lc != "revoked") {
            printf "%s\t%s\t%s\n", email, pass, status >> active_file
        }
        delete inbound[email]
    }
    END {
        for (email in inbound) {
            printf "missing_in_clients\t%s\t%s\n", email, inbound[email] >> mismatch_file
            mismatch = 1
        }
        if (mismatch) {
            exit 1
        }
    }
' "$INBOUND_TMP" "$CLIENTS_TMP"; then
    awk_status=$?
fi

TAB_CHAR=$(printf '\t')

if [ -s "$MISMATCH_TMP" ]; then
    printf 'Error: Clients registry and trojan inbound configuration differ.\n' >&2
    while IFS="$TAB_CHAR" read -r type email val1 val2; do
        case "$type" in
            missing_in_inbounds)
                printf ' - %s exists in %s but is absent from trojan inbound list.\n' "$email" "$CLIENTS_FILE" >&2
                ;;
            missing_in_clients)
                printf ' - %s exists in %s but is absent from %s.\n' "$email" "$INBOUNDS_FILE" "$CLIENTS_FILE" >&2
                ;;
            password_mismatch)
                printf ' - %s password mismatch: %s contains %s, %s contains %s.\n' \
                    "$email" "$CLIENTS_FILE" "$val1" "$INBOUNDS_FILE" "$val2" >&2
                ;;
            *)
                printf ' - %s %s %s %s\n' "$type" "$email" "$val1" "$val2" >&2
                ;;
        esac
    done < "$MISMATCH_TMP"
    exit 1
fi

if [ "$awk_status" -ne 0 ]; then
    die "Failed to compare client registry with inbound configuration."
fi

if ! [ -s "$ACTIVE_TMP" ]; then
    printf 'No active clients found (status not equal to revoked).\n'
    exit 0
fi

sort -t "$TAB_CHAR" -k1,1 "$ACTIVE_TMP" |
while IFS="$TAB_CHAR" read -r email password status; do
    printf '%s %s %s\n' "$email" "$password" "$status"
done
