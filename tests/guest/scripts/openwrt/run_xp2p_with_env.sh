#!/bin/sh
set -eu

if ! command -v xp2p >/dev/null 2>&1; then
  echo "xp2p executable not found in PATH" >&2
  exit 1
fi

while [ "$#" -gt 0 ]; do
  if [ "$1" = "--" ]; then
    shift
    break
  fi
  case "$1" in
    *=*)
      export "$1"
      shift
      ;;
    *)
      break
      ;;
  esac
done

if [ "$#" -eq 0 ]; then
  echo "xp2p command arguments are required" >&2
  exit 1
fi

exec xp2p "$@"
