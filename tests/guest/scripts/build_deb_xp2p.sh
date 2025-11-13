#!/bin/sh
set -eu

script_dir=$(cd "$(dirname "$0")" && pwd)
PROJECT_ROOT=${XP2P_PROJECT_ROOT:-$(cd "$script_dir/../../.." && pwd)}
BUILD_ROOT=${XP2P_DEB_BUILD_ROOT:-"$PROJECT_ROOT/build/deb"}
STAGING_DIR="$BUILD_ROOT/staging"
ARTIFACT_DIR="$BUILD_ROOT/artifacts"
PKG_NAME=${XP2P_DEB_NAME:-xp2p}
PKG_ARCH=${XP2P_DEB_ARCH:-amd64}
VERSION_SOURCE=${XP2P_VERSION_SOURCE:-"./go/cmd/xp2p"}
DEPENDS=${XP2P_DEB_DEPENDS:-"xray-core"}
DESCRIPTION=${XP2P_DEB_DESCRIPTION:-"xp2p Trojan tunnel CLI and helpers"}
HOMEPAGE=${XP2P_DEB_URL:-"https://github.com/NlightN22/xray-p2p"}
MAINTAINER=${XP2P_DEB_MAINTAINER:-"xp2p maintainers <maintainers@xray-p2p>"}
LICENSE=${XP2P_DEB_LICENSE:-"Proprietary"}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1" >&2
    exit 1
  fi
}

require_cmd go
require_cmd fpm

mkdir -p "$ARTIFACT_DIR"

echo "==> Determining xp2p version"
VERSION=$(
  cd "$PROJECT_ROOT"
  go run "$VERSION_SOURCE" --version | tr -d '[:space:]'
)

if [ -z "$VERSION" ]; then
  echo "Unable to determine xp2p version" >&2
  exit 1
fi

echo "==> Version: $VERSION"
echo "==> Preparing staging tree at $STAGING_DIR"
rm -rf "$STAGING_DIR"
mkdir -p \
  "$STAGING_DIR/usr/sbin" \
  "$STAGING_DIR/etc/xp2p/config-client" \
  "$STAGING_DIR/etc/xp2p/config-server" \
  "$STAGING_DIR/var/log/xp2p"

echo "==> Building xp2p binary (GOARCH=$PKG_ARCH)"
LDFLAGS="-s -w -X github.com/NlightN22/xray-p2p/go/internal/version.current=${VERSION}"
(
  cd "$PROJECT_ROOT"
  env GOOS=linux GOARCH="$PKG_ARCH" CGO_ENABLED=0 \
    go build -ldflags "$LDFLAGS" -o "$STAGING_DIR/usr/sbin/xp2p" ./go/cmd/xp2p
)
chmod 0755 "$STAGING_DIR/usr/sbin/xp2p"

PACKAGE_PATH="$ARTIFACT_DIR/${PKG_NAME}_${VERSION}_${PKG_ARCH}.deb"

if [ -e "$PACKAGE_PATH" ]; then
  echo "==> Removing existing package at $PACKAGE_PATH"
  rm -f "$PACKAGE_PATH"
fi

echo "==> Packaging $PACKAGE_PATH"

set --
for dep in $DEPENDS; do
  if [ -n "$dep" ]; then
    set -- "$@" --depends "$dep"
  fi
done

fpm -s dir -t deb \
  -n "$PKG_NAME" \
  -v "$VERSION" \
  --architecture "$PKG_ARCH" \
  --description "$DESCRIPTION" \
  --url "$HOMEPAGE" \
  --maintainer "$MAINTAINER" \
  --license "$LICENSE" \
  "$@" \
  --package "$PACKAGE_PATH" \
  -C "$STAGING_DIR" \
  usr etc var

echo "==> Package ready: $PACKAGE_PATH"
