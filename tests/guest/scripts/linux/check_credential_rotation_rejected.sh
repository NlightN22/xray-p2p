#!/bin/sh
set -eu
script_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
exec /usr/bin/python3 "$script_dir/check_credential_rotation_rejected.py" "$@"
