#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
PROJECT_ROOT=$(realpath "${SCRIPT_DIR}/../../../..")
STAGING_ROOT="${PROJECT_ROOT}/build/deb"
STAGING_DIR="${STAGING_ROOT}/staging"
OUTPUT_DIR="${STAGING_ROOT}/artifacts"
PKG_NAME=${PKG_NAME:-xp2p}
PKG_ARCH=${PKG_ARCH:-amd64}
VERSION_SOURCE=${VERSION_SOURCE:-"./go/cmd/xp2p"}

command -v go >/dev/null 2>&1 || { echo "go is not installed"; exit 1; }
command -v fpm >/dev/null 2>&1 || { echo "fpm is not installed"; exit 1; }

echo "Resolving xp2p version..."
VERSION=$(cd "$PROJECT_ROOT" && go run "$VERSION_SOURCE" --version | tr -d '[:space:]')
if [ -z "$VERSION" ]; then
  echo "Failed to determine xp2p version" >&2
  exit 1
fi
echo "Version detected: ${VERSION}"

echo "Preparing staging directory..."
rm -rf "$STAGING_DIR"
mkdir -p \
  "$STAGING_DIR/usr/sbin" \
  "$STAGING_DIR/etc/xp2p/config-client" \
  "$STAGING_DIR/etc/xp2p/config-server" \
  "$STAGING_DIR/var/log/xp2p"

echo "Building xp2p binary..."
LDFLAGS="-s -w -X github.com/NlightN22/xray-p2p/go/internal/version.current=${VERSION}"
(
  cd "$PROJECT_ROOT"
  env GOOS=linux GOARCH="$PKG_ARCH" CGO_ENABLED=0 \
    go build -ldflags "$LDFLAGS" -o "$STAGING_DIR/usr/sbin/xp2p" ./go/cmd/xp2p
)

chmod 0755 "$STAGING_DIR/usr/sbin/xp2p"

mkdir -p "$OUTPUT_DIR"
PACKAGE_PATH="${OUTPUT_DIR}/${PKG_NAME}_${VERSION}_${PKG_ARCH}.deb"

echo "Packaging ${PACKAGE_PATH}..."
fpm -s dir -t deb \
  -n "$PKG_NAME" \
  -v "$VERSION" \
  --architecture "$PKG_ARCH" \
  --description "xp2p Trojan tunnel CLI and helpers" \
  --url "https://github.com/NlightN22/xray-p2p" \
  --maintainer "xp2p maintainers <maintainers@xray-p2p>" \
  --license "Proprietary" \
  --depends "ca-certificates" \
  --depends "iptables" \
  --depends "iproute2" \
  --package "$PACKAGE_PATH" \
  -C "$STAGING_DIR" \
  usr etc var

echo "Package ready at: $PACKAGE_PATH"
