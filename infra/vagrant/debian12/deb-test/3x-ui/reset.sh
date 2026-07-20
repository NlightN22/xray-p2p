#!/bin/sh
set -eu

script_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
cd "$script_dir"

docker-compose down --remove-orphans --volumes
state_dir="$script_dir/state"
case "$state_dir" in
  "$script_dir"/state) rm -rf -- "$state_dir" ;;
  *) echo "Refusing to remove unexpected state path" >&2; exit 1 ;;
esac
mkdir -m 700 "$state_dir"
