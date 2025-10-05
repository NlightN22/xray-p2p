#!/bin/sh
################### upd
set -eu

SCRIPT_NAME=${0##*/}

usage() {
    cat <<EOF_USAGE
Usage: $SCRIPT_NAME [options] [USERNAME]

Ensure the reverse proxy routing is configured for USERNAME on the server.

Options:
  -h, --help        Show this help message and exit.
  -s, --subnet CIDR Add CIDR subnet to server reverse routing (repeatable).

Arguments:
  USERNAME          Optional XRAY client identifier; overrides env/prompt.

Environment variables:
  XRAY_REVERSE_USER         Preseed USERNAME when positional argument omitted.
  XRAY_REVERSE_SUFFIX       Override domain/tag suffix (default: .rev).
  XRAY_REVERSE_SUBNETS      Default comma/space separated subnets for server target.
  XRAY_REVERSE_SUBNET       Single default subnet (alias for XRAY_REVERSE_SUBNETS).
  XRAY_CONFIG_DIR           Path to XRAY configuration directory (default: /etc/xray).
  XRAY_ROUTING_FILE         Path to routing.json (defaults to ${XRAY_CONFIG_DIR:-/etc/xray}/routing.json).
  XRAY_ROUTING_TEMPLATE     Optional local template path for routing.json.
  XRAY_ROUTING_TEMPLATE_REMOTE  Optional remote template path relative to repo root.
EOF_USAGE
    exit "${1:-0}"
}

trim_spaces() {
    printf '%s' "$1" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//'
}

append_subnet() {
    candidate=$(trim_spaces "$1")
    if [ -z "$candidate" ]; then
        return
    fi

    case "
$reverse_subnets
" in
        *"
$candidate
"*)
            return
            ;;
    esac

    if [ -n "$reverse_subnets" ]; then
        reverse_subnets="$reverse_subnets
$candidate"
    else
        reverse_subnets="$candidate"
    fi
}

add_subnets_from_string() {
    list_input="$1"
    if [ -z "$list_input" ]; then
        return
    fi

    sanitized=$(printf '%s' "$list_input" | tr ',' ' ')
    for entry in $sanitized; do
        trimmed=$(trim_spaces "$entry")
        if [ -z "$trimmed" ]; then
            continue
        fi
        if ! validate_subnet "$trimmed"; then
            xray_die "Invalid subnet: $trimmed"
        fi
        append_subnet "$trimmed"
    done
}

username_arg=""
reverse_subnets=""

add_subnets_from_string "${XRAY_REVERSE_SUBNETS:-}"
add_subnets_from_string "${XRAY_REVERSE_SUBNET:-}"

while [ "$#" -gt 0 ]; do
    case "$1" in
        -h|--help)
            usage 0
            ;;
        -s|--subnet)
            if [ -n "${2:-}" ]; then
                cli_subnet=$(trim_spaces "$2")
                if [ -z "$cli_subnet" ]; then
                    xray_die "Invalid subnet: (empty)"
                fi
                if ! validate_subnet "$cli_subnet"; then
                    xray_die "Invalid subnet: $cli_subnet"
                fi
                append_subnet "$cli_subnet"
                shift
            else
                printf 'Option %s requires an argument.\n' "$1" >&2
                usage 1
            fi
            ;;
        --subnet=*)
            value=$(trim_spaces "${1#*=}")
            if [ -z "$value" ]; then
                xray_die "Invalid subnet: (empty)"
            fi
            if ! validate_subnet "$value"; then
                xray_die "Invalid subnet: $value"
            fi
            append_subnet "$value"
            ;;
        --)
            shift
            break
            ;;
        -*)
            printf 'Unknown option: %s\n' "$1" >&2
            usage 1
            ;;
        *)
            if [ -z "$username_arg" ]; then
                username_arg="$1"
            else
                printf 'Unexpected argument: %s\n' "$1" >&2
                usage 1
            fi
            ;;
    esac
    shift
done

if [ "$#" -gt 0 ]; then
    printf 'Unexpected argument: %s\n' "$1" >&2
    usage 1
fi

if [ -z "${XRAY_SELF_DIR:-}" ]; then
    case "$0" in
        */*)
            XRAY_SELF_DIR=$(CDPATH= cd -- "$(dirname "$0")" 2>/dev/null && pwd)
            export XRAY_SELF_DIR
            ;;
    esac
fi

# Ensure XRAY_SELF_DIR exists when invoked via stdin piping.
: "${XRAY_SELF_DIR:=}"

umask 077

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

if ! load_common_lib; then
    printf 'Error: Unable to load XRAY common library.\n' >&2
    exit 1
fi

CONFIG_DIR="${XRAY_CONFIG_DIR:-/etc/xray}"
ROUTING_FILE="${XRAY_ROUTING_FILE:-$CONFIG_DIR/routing.json}"

ROUTING_TEMPLATE_REMOTE="${XRAY_ROUTING_TEMPLATE_REMOTE:-}"
ROUTING_TEMPLATE_LOCAL="${XRAY_ROUTING_TEMPLATE:-}"

xray_require_cmd jq

ensure_routing_file() {
    if [ -f "$ROUTING_FILE" ]; then
        return
    fi

    if [ -n "$ROUTING_TEMPLATE_REMOTE" ]; then
        xray_seed_file_from_template "$ROUTING_FILE" "$ROUTING_TEMPLATE_REMOTE" "${ROUTING_TEMPLATE_LOCAL:-$ROUTING_TEMPLATE_REMOTE}"
        return
    fi

    if [ -n "$ROUTING_TEMPLATE_LOCAL" ]; then
        if resolved_template=$(xray_resolve_local_path "$ROUTING_TEMPLATE_LOCAL"); then
            template_path="$resolved_template"
        else
            template_path="$resolved_template"
        fi

        if [ -n "$template_path" ] && [ -r "$template_path" ]; then
            if ! xray_should_replace_file "$ROUTING_FILE" "XRAY_FORCE_CONFIG"; then
                return
            fi
            dest_dir=$(dirname "$ROUTING_FILE")
            if [ ! -d "$dest_dir" ]; then
                mkdir -p "$dest_dir" || xray_die "Unable to create directory $dest_dir"
            fi
            if ! cp "$template_path" "$ROUTING_FILE"; then
                xray_die "Failed to copy template from $template_path"
            fi
            chmod 0644 "$ROUTING_FILE" 2>/dev/null || true
            xray_log "Seeded $ROUTING_FILE from local template $template_path"
            return
        fi
    fi

    tmp="$(mktemp 2>/dev/null)" || xray_die "Unable to create temporary file"
    cat >"$tmp" <<'EOF'
{
    "reverse": {
        "portals": []
    },
    "routing": {
        "domainStrategy": "AsIs",
        "rules": []
    }
}
EOF
    chmod 0644 "$tmp"
    dest_dir=$(dirname "$ROUTING_FILE")
    if [ ! -d "$dest_dir" ]; then
        mkdir -p "$dest_dir" || {
            rm -f "$tmp"
            xray_die "Unable to create directory $dest_dir"
        }
    fi
    mv "$tmp" "$ROUTING_FILE"
    xray_log "Generated default routing config at $ROUTING_FILE"
}

run_user_list() {
    xray_log "Existing XRAY users (user_list.sh, email password status):"
    if ! xray_run_repo_script optional "lib/user_list.sh" "scripts/lib/user_list.sh"; then
        xray_log "Unable to execute user_list.sh; continuing without user listing."
    fi
}

read_username() {
    value="$1"
    if [ -n "$value" ]; then
        printf '%s' "$value"
        return
    fi

    if [ -n "${XRAY_REVERSE_USER:-}" ]; then
        printf '%s' "$XRAY_REVERSE_USER"
        return
    fi

    read_fd=0
    if [ ! -t 0 ]; then
        if [ -r /dev/tty ]; then
            exec 3</dev/tty
            read_fd=3
        else
            xray_die "Username argument required; no interactive terminal available"
        fi
    fi

    while :; do
        printf 'Enter XRAY username: ' >&2
        if [ "$read_fd" -eq 3 ]; then
            IFS= read -r input <&3 || input=""
        else
            IFS= read -r input || input=""
        fi
        if [ -n "$input" ]; then
            printf '%s' "$input"
            return
        fi
        xray_log "Username cannot be empty."
    done
}

validate_username() {
    candidate="$1"
    case "$candidate" in
        ''|*[!A-Za-z0-9._-]*)
            xray_die "Username must contain only letters, digits, dot, underscore, or dash"
            ;;
    esac
}

prompt_subnets_if_needed() {
    if [ -n "$reverse_subnets" ]; then
        return
    fi

    read_fd=0
    if [ ! -t 0 ]; then
        if [ -r /dev/tty ]; then
            exec 4</dev/tty
            read_fd=4
        else
            return
        fi
    fi

    xray_log "No CIDR subnets supplied; press Enter to skip or provide one per prompt."

    while :; do
        printf 'Enter CIDR subnet for reverse routing (blank to finish): ' >&2
        if [ "$read_fd" -eq 4 ]; then
            IFS= read -r input <&4 || input=""
        else
            IFS= read -r input || input=""
        fi

        trimmed=$(trim_spaces "$input")
        if [ -z "$trimmed" ]; then
            break
        fi

        if ! validate_subnet "$trimmed"; then
            xray_log "Invalid subnet, expected CIDR (e.g. 10.0.102.0/24)."
            continue
        fi

        append_subnet "$trimmed"
    done

    if [ "$read_fd" -eq 4 ]; then
        exec 4<&-
    fi
}

update_routing_server() {
    username="$1"
    suffix="${XRAY_REVERSE_SUFFIX:-.rev}"
    domain="$username$suffix"
    tag="$domain"

    tmp="$(mktemp 2>/dev/null)" || xray_die "Unable to create temporary file"

    subnet_json='[]'
    if [ -n "$reverse_subnets" ]; then
        subnet_json=$(printf '%s' "$reverse_subnets" | jq -Rsc 'split("\n") | map(select(length > 0))') || {
            rm -f "$tmp"
            xray_die "Failed to encode subnet list"
        }
    fi

    if ! jq \
        --arg user "$username" \
        --arg domain "$domain" \
        --arg tag "$tag" \
        --argjson subnets "$subnet_json" \
        '
        def ensure_array:
            if . == null then []
            elif type == "array" then .
            else [.] end;

        def has_domain_match($rule_domain):
            ($rule_domain | ensure_array | index("full:" + $domain)) != null;

        def has_subnet_match($rule_ip):
            if ($subnets | length) == 0 then
                false
            else
                ($rule_ip | ensure_array) as $ips
                | any($subnets[]; ($ips | index(.)) != null)
            end;

        .reverse = (.reverse // {}) |
        .reverse.portals = (
            (.reverse.portals // [])
            | [ .[] | select(.domain != $domain) ]
            + [{ domain: $domain, tag: $tag }]
        ) |
        .routing = (.routing // {}) |
        (.routing.rules // []) as $rules |
        .routing.rules = (
            [ $rules[] | select((
                (.outboundTag == $tag and (has_domain_match(.domain) or has_subnet_match(.ip))) | not
            )) ]
            + [{
                type: "field",
                domain: ["full:" + $domain],
                outboundTag: $tag,
                user: [$user]
            }]
            + (if ($subnets | length) > 0 then [
                {
                    type: "field",
                    ip: $subnets,
                    outboundTag: $tag
                }
            ] else [] end)
        )
        ' "$ROUTING_FILE" >"$tmp"; then
        rm -f "$tmp"
        xray_die "Failed to update $ROUTING_FILE"
    fi

    chmod 0644 "$tmp"
    mv "$tmp" "$ROUTING_FILE"

    xray_log "Updated $ROUTING_FILE with reverse proxy entry for $username (tag: $tag)"
}

ensure_routing_file
run_user_list

USERNAME=$(read_username "$username_arg")
validate_username "$USERNAME"
prompt_subnets_if_needed
update_routing_server "$USERNAME"
xray_restart_service "" "" ""

xray_log "Reverse proxy server install complete."
