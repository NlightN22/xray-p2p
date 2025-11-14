#!/bin/bash
set -euo pipefail

WORK_TREE=${XP2P_PROJECT_ROOT:-/srv/xray-p2p}
BUILD_SCRIPT="$WORK_TREE/scripts/build/build_deb_xp2p.sh"
ARTIFACT_DIR="$WORK_TREE/build/deb/artifacts"
INSTALL_BIN="/usr/bin/xp2p"

emit_versions() {
  echo "__XP2P_SOURCE_VERSION__=${1:-}"
  echo "__XP2P_INSTALLED_VERSION__=${2:-}"
}

if [ ! -d "$WORK_TREE" ]; then
  echo "Missing xp2p repo at $WORK_TREE" >&2
  emit_versions ""
  exit 3
fi

if [ ! -x "$BUILD_SCRIPT" ]; then
  echo "Build script $BUILD_SCRIPT is not executable" >&2
  emit_versions ""
  exit 3
fi

export PATH="/usr/local/go/bin:$PATH"
cd "$WORK_TREE"

SOURCE_VERSION=$(go run ./go/cmd/xp2p --version | tr -d '\r')
if [ -z "$SOURCE_VERSION" ]; then
  echo "Unable to determine xp2p source version" >&2
  emit_versions ""
  exit 3
fi

INSTALLED_VERSION=""
if [ -x "$INSTALL_BIN" ]; then
  INSTALLED_VERSION=$("$INSTALL_BIN" --version | tr -d '\r')
fi

if [ "$INSTALLED_VERSION" != "$SOURCE_VERSION" ]; then
  "$BUILD_SCRIPT"
  ARCH=$(dpkg --print-architecture)
  shopt -s nullglob
  LATEST_PKG=""
  for pkg in "$ARTIFACT_DIR"/xp2p_*_"$ARCH".deb; do
    if [ -z "$LATEST_PKG" ] || [ "$pkg" -nt "$LATEST_PKG" ]; then
      LATEST_PKG="$pkg"
    fi
  done
  shopt -u nullglob
  if [ -z "$LATEST_PKG" ]; then
    echo "xp2p package not found in $ARTIFACT_DIR for arch $ARCH" >&2
    emit_versions ""
    exit 3
  fi
  sudo dpkg -i "$LATEST_PKG" >/dev/null
  INSTALLED_VERSION=$("$INSTALL_BIN" --version | tr -d '\r')
fi

emit_versions "$SOURCE_VERSION" "$INSTALLED_VERSION"
