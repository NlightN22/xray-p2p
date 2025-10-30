#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "Usage: $0 <version> <artifact tar.gz>" >&2
  exit 1
fi

VERSION="$1"
ARCHIVE="$2"

if [ ! -f "$ARCHIVE" ]; then
  echo "Artifact archive '$ARCHIVE' not found" >&2
  exit 1
fi

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DIST_DIR="$REPO_ROOT/dist/ipk"
STAGING_DIR="$(mktemp -d)"
PKG_BUILD_DIR="$(mktemp -d)"

cleanup() {
  rm -rf "$STAGING_DIR" "$PKG_BUILD_DIR"
}
trap cleanup EXIT

mkdir -p "$DIST_DIR"

tar -xzf "$ARCHIVE" -C "$STAGING_DIR"

BIN_PATH="$(find "$STAGING_DIR" -maxdepth 1 -type f -name 'xp2p*' | head -n 1)"
if [ -z "$BIN_PATH" ]; then
  echo "xp2p binary not found in archive '$ARCHIVE'" >&2
  exit 1
fi

PKG_DIR="$PKG_BUILD_DIR/xp2p"
mkdir -p "$PKG_DIR/CONTROL" \
         "$PKG_DIR/usr/bin" \
         "$PKG_DIR/etc/xp2p"

install -m 0755 "$BIN_PATH" "$PKG_DIR/usr/bin/xp2p"

if [ -f "$REPO_ROOT/config_templates/xp2p.example.yaml" ]; then
  install -m 0644 "$REPO_ROOT/config_templates/xp2p.example.yaml" "$PKG_DIR/etc/xp2p/xp2p.example.yaml"
fi

cat >"$PKG_DIR/CONTROL/control" <<EOF
Package: xp2p
Version: ${VERSION}
Architecture: x86_64
Maintainer: xrAy-p2p maintainers
Section: net
Priority: optional
Description: XRAY P2P helper binary for diagnostics and ping utilities.
EOF

opkg-build "$PKG_DIR" "$DIST_DIR" >/dev/null

echo "Built package(s):"
ls -1 "$DIST_DIR"/*.ipk
