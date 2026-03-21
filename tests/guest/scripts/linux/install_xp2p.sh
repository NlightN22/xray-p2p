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

if [ ! -f "$BUILD_SCRIPT" ]; then
  echo "Build script $BUILD_SCRIPT is missing" >&2
  emit_versions ""
  exit 3
fi

export PATH="/usr/local/go/bin:$PATH"
cd "$WORK_TREE"
sudo -n systemctl stop xp2p-client xp2p-server >/dev/null 2>&1 || true
sudo -n pkill -f '/etc/xp2p/bin/xray' >/dev/null 2>&1 || true
sudo -n pkill -f '/srv/xray-p2p/build/deb/staging/etc/xp2p/bin/xray' >/dev/null 2>&1 || true
sudo -n rm -rf /srv/xray-p2p/build/deb/staging >/dev/null 2>&1 || true

SOURCE_VERSION=$(go run ./go/cmd/xp2p --version | tr -d '\r')
if [ -z "$SOURCE_VERSION" ]; then
  echo "Unable to determine xp2p source version" >&2
  emit_versions ""
  exit 3
fi
ARCH=$(dpkg --print-architecture)
EXPECTED_PKG="$ARTIFACT_DIR/xp2p_${SOURCE_VERSION}_${ARCH}.deb"
SKIP_BUILD="${XP2P_SKIP_BUILD:-0}"
LATEST_PKG=""
echo "==> Purging existing xp2p package"
sudo -n dpkg -P xp2p >/dev/null 2>&1 || true
if [ "$SKIP_BUILD" = "1" ]; then
  if [ ! -f "$EXPECTED_PKG" ]; then
    echo "Expected cached package not found: $EXPECTED_PKG" >&2
    emit_versions ""
    exit 3
  fi
  echo "==> Using cached package $EXPECTED_PKG"
  LATEST_PKG="$EXPECTED_PKG"
else
  bash "$BUILD_SCRIPT"
fi
shopt -s nullglob
if [ -z "$LATEST_PKG" ]; then
  for pkg in "$ARTIFACT_DIR"/xp2p_*_"$ARCH".deb; do
    if [ -z "$LATEST_PKG" ] || [ "$pkg" -nt "$LATEST_PKG" ]; then
      LATEST_PKG="$pkg"
    fi
  done
fi
shopt -u nullglob
if [ -z "$LATEST_PKG" ]; then
  echo "xp2p package not found in $ARTIFACT_DIR for arch $ARCH" >&2
  emit_versions ""
  exit 3
fi

install_package() {
  local attempts=0
  local max_attempts=30
  local log_file
  log_file=$(mktemp)
  while true; do
    if sudo dpkg -i "$LATEST_PKG" >"$log_file" 2>&1; then
      rm -f "$log_file"
      return 0
    fi
    if grep -qi "frontend lock" "$log_file"; then
      attempts=$((attempts + 1))
      if [ "$attempts" -ge "$max_attempts" ]; then
        cat "$log_file" >&2
        rm -f "$log_file"
        return 1
      fi
      sleep 2
      continue
    fi
    cat "$log_file" >&2
    rm -f "$log_file"
    return 1
  done
}

install_package
if [ ! -x "$INSTALL_BIN" ]; then
  echo "xp2p binary is not executable: $(ls -l "$INSTALL_BIN" 2>/dev/null || true)" >&2
  sudo -n chmod +x "$INSTALL_BIN" >/dev/null 2>&1 || true
fi
if [ ! -x "$INSTALL_BIN" ]; then
  echo "xp2p binary is still not executable: $(ls -l "$INSTALL_BIN" 2>/dev/null || true)" >&2
  emit_versions "$SOURCE_VERSION" ""
  exit 3
fi
INSTALLED_VERSION=$("$INSTALL_BIN" --version | tr -d '\r')

emit_versions "$SOURCE_VERSION" "$INSTALLED_VERSION"
