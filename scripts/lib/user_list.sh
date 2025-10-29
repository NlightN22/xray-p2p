#!/bin/sh

# Library module for generating XRAY user listings.
# When sourced by another script set XRAY_USER_LIST_SKIP_MAIN=1 to
# prevent automatic execution of the standalone entry point.

SCRIPT_NAME=${0##*/}

if [ -z "${XRAY_SELF_DIR:-}" ]; then
    case "$0" in
        */*)
            XRAY_SELF_DIR=$(CDPATH= cd -- "$(dirname "$0")" 2>/dev/null && pwd)
            export XRAY_SELF_DIR
            ;;
    esac
fi

# Ensure XRAY_SELF_DIR exists even when invoked via stdin piping.
: "${XRAY_SELF_DIR:=}"

COMMON_LIB_REMOTE_PATH="scripts/lib/common.sh"

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

xray_user_list_main() {
    CONFIG_DIR="${XRAY_CONFIG_DIR:-/etc/xray-p2p}"
    INBOUNDS_FILE="${XRAY_INBOUNDS_FILE:-$CONFIG_DIR/inbounds.json}"
    CLIENTS_DIR="${XRAY_CLIENTS_DIR:-$CONFIG_DIR/config}"
    CLIENTS_FILE="${XRAY_CLIENTS_FILE:-$CLIENTS_DIR/clients.json}"
    format="${XRAY_OUTPUT_MODE:-table}"

    while [ "$#" -gt 0 ]; do
        if xray_consume_json_flag "$@"; then
            shift "$XRAY_JSON_FLAG_CONSUMED"
            continue
        fi
        printf 'Unexpected argument: %s\n' "$1" >&2
        return 1
    done

    xray_require_cmd jq

    xray_check_repo_access 'scripts/lib/user_list.sh'

    if [ ! -f "$CLIENTS_FILE" ]; then
        xray_warn "Clients registry not found: $CLIENTS_FILE"
        if [ "$format" = "json" ]; then
            printf '[]\n'
        fi
        return 0
    fi

    if [ ! -f "$INBOUNDS_FILE" ]; then
        xray_die "Inbound configuration not found: $INBOUNDS_FILE"
    fi

    if ! jq empty "$CLIENTS_FILE" >/dev/null 2>&1; then
        xray_die "Clients registry $CLIENTS_FILE contains invalid JSON."
    fi

    if ! jq empty "$INBOUNDS_FILE" >/dev/null 2>&1; then
        xray_die "Inbound configuration $INBOUNDS_FILE contains invalid JSON."
    fi

    CLIENTS_TMP=$(mktemp) || xray_die "Unable to create temporary file"
    INBOUND_TMP=$(mktemp) || xray_die "Unable to create temporary file"
    MISMATCH_TMP=$(mktemp) || xray_die "Unable to create temporary file"
    ACTIVE_TMP=$(mktemp) || xray_die "Unable to create temporary file"

    trap 'rm -f "$CLIENTS_TMP" "$INBOUND_TMP" "$MISMATCH_TMP" "$ACTIVE_TMP"' EXIT INT TERM

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
        return 1
    fi

    if [ "$awk_status" -ne 0 ]; then
        xray_die "Failed to compare client registry with inbound configuration."
    fi

    if ! [ -s "$ACTIVE_TMP" ]; then
        if [ "$format" = "json" ]; then
            printf '[]\n'
        else
            printf 'No active clients found (status not equal to revoked).\n'
        fi
        return 0
    fi

    sort -t "$TAB_CHAR" -k1,1 "$ACTIVE_TMP" |
    {
        printf 'Email\tPassword\tStatus\n'
        while IFS="$TAB_CHAR" read -r email password status; do
            printf '%s\t%s\t%s\n' "$email" "$password" "$status"
        done
    } | xray_print_table
}

if [ "${XRAY_USER_LIST_SKIP_MAIN:-0}" != "1" ]; then
    if ! command -v xray_log >/dev/null 2>&1; then
        if ! load_common_lib; then
            printf 'Error: Unable to load XRAY common library.\n' >&2
            exit 1
        fi
    fi
    xray_user_list_main "$@"
    exit $?
fi
