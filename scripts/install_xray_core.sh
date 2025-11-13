#!/bin/sh
set -eu

if [ "$(id -u)" -ne 0 ]; then
  echo "This script must be run as root." >&2
  exit 1
fi

TMP_DIR=$(mktemp -d)
cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

INSTALLER_URL=${XRAY_INSTALLER_URL:-"https://raw.githubusercontent.com/XTLS/Xray-install/main/install-release.sh"}
INSTALLER_PATH="$TMP_DIR/install-release.sh"

echo "==> Downloading Xray installer from $INSTALLER_URL"
curl -fsSL "$INSTALLER_URL" -o "$INSTALLER_PATH"
chmod +x "$INSTALLER_PATH"

echo "==> Running upstream installer"
exec bash "$INSTALLER_PATH" "$@"
