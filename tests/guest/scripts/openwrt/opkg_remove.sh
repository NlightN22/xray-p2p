#!/bin/sh
set -eu

PACKAGE=""
IGNORE_MISSING=0

usage() {
  cat <<'USAGE'
Usage: opkg_remove.sh --package <name> [--ignore-missing]

Options:
  --package <name>    Package to remove via opkg
  --ignore-missing    Treat absent packages as success
  -h, --help          Show this message
USAGE
}

while [ "${1:-}" != "" ]; do
  case "$1" in
    --package)
      PACKAGE="$2"
      shift 2
      ;;
    --ignore-missing)
      IGNORE_MISSING=1
      shift
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

if [ -z "$PACKAGE" ]; then
  echo "ERROR: --package is required" >&2
  exit 1
fi

if opkg status "$PACKAGE" >/dev/null 2>&1; then
  exec opkg remove "$PACKAGE"
fi

if [ "$IGNORE_MISSING" -eq 1 ]; then
  echo "Package $PACKAGE not installed; skipping removal." >&2
  exit 0
fi

echo "Package $PACKAGE is not installed" >&2
exit 1
