#!/bin/sh
set -eu

IPK_PATH=""

usage() {
  cat <<'USAGE'
Usage: opkg_install_local.sh --path <file.ipk>

Options:
  --path <file.ipk>   Local ipk path to install via opkg
  -h, --help          Show this message
USAGE
}

while [ "${1:-}" != "" ]; do
  case "$1" in
    --path)
      IPK_PATH="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage
      exit 1
      ;;
  esac
done

if [ -z "$IPK_PATH" ]; then
  echo "ERROR: --path is required" >&2
  exit 1
fi

if [ ! -f "$IPK_PATH" ]; then
  echo "ERROR: ipk path $IPK_PATH does not exist" >&2
  exit 1
fi

exec opkg install --force-reinstall "$IPK_PATH"
