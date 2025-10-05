#!/bin/sh
set -eu

self_path="$0"
case "$self_path" in
    */*)
        script_path="$self_path"
        ;;
    *)
        if resolved_path=$(command -v -- "$self_path" 2>/dev/null); then
            script_path="$resolved_path"
        else
            script_path="$self_path"
        fi
        ;;
esac

script_dir=$(CDPATH= cd -- "$(dirname "$script_path")" 2>/dev/null && pwd)

export XRAY_REVERSE_TARGET="${XRAY_REVERSE_TARGET:-client}"

exec "$script_dir/server_reverse_add.sh" "$@"
