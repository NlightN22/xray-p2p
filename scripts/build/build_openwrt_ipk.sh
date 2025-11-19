#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
PROJECT_ROOT=$(cd "$SCRIPT_DIR/../.." && pwd)
DEFAULT_BUILD_ROOT="/tmp/build"

TARGET=""
SDK_DIR=""
DIFFCONFIG=""
DIFFCONFIG_OUT=""
BUILD_ROOT=${XP2P_BUILD_ROOT:-$DEFAULT_BUILD_ROOT}
FEED_PATH="$PROJECT_ROOT/openwrt/feed"
FEED_PACKAGE_PATH="$FEED_PATH/packages/utils/xp2p"
REPO_ROOT="$PROJECT_ROOT/openwrt/repo"
RELEASE_VERSION=${OPENWRT_VERSION:-""}
GOTOOLCHAIN_VERSION=${GOTOOLCHAIN:-go1.21.7}

usage() {
  cat <<'EOF'
Usage: build_openwrt_ipk.sh --target <identifier> [options]

Options:
  --sdk-dir <path>         Existing OpenWrt SDK directory (defaults to ~/openwrt-sdk-<target>)
  --diffconfig <path>      diffconfig file applied before make defconfig
  --diffconfig-out <path>  write fresh diffconfig after defconfig
  --build-root <path>      directory containing prebuilt xp2p/xray/completions (default: /tmp/build)
  -h, --help               Show this message
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --target)
      TARGET="$2"
      shift 2
      ;;
    --sdk-dir)
      SDK_DIR="$2"
      shift 2
      ;;
    --diffconfig)
      DIFFCONFIG="$2"
      shift 2
      ;;
    --diffconfig-out)
      DIFFCONFIG_OUT="$2"
      shift 2
      ;;
    --build-root)
      BUILD_ROOT="$2"
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

if [ -z "$TARGET" ]; then
  echo "ERROR: --target is required" >&2
  usage
  exit 1
fi

if [ -z "$SDK_DIR" ]; then
  SDK_DIR="$HOME/openwrt-sdk-$TARGET"
fi

OUTPUT_DIR="${BUILD_ROOT%/}/$TARGET"
XP2P_BIN="$OUTPUT_DIR/xp2p"
XRAY_BIN="$OUTPUT_DIR/xray"
COMPLETIONS_DIR="$OUTPUT_DIR/completions"

echo "==> [$TARGET] Ensuring OpenWrt SDK"
"$PROJECT_ROOT/scripts/build/ensure_openwrt_sdk.sh" "$TARGET"

if [ -z "$RELEASE_VERSION" ]; then
  if [ -f "$SDK_DIR/.xp2p-openwrt-version" ]; then
    RELEASE_VERSION=$(cut -d'-' -f1 "$SDK_DIR/.xp2p-openwrt-version")
  else
    RELEASE_VERSION="unknown"
  fi
fi

echo "==> [$TARGET] Building xp2p binaries"
GOTOOLCHAIN=$GOTOOLCHAIN_VERSION "$PROJECT_ROOT/scripts/build/build_xp2p_binaries.sh" --target "$TARGET"

if [ ! -x "$XP2P_BIN" ]; then
  echo "ERROR: xp2p binary not found at $XP2P_BIN" >&2
  exit 1
fi
if [ ! -x "$XRAY_BIN" ]; then
  echo "ERROR: bundled xray not found at $XRAY_BIN" >&2
  exit 1
fi
if [ ! -d "$COMPLETIONS_DIR" ]; then
  echo "ERROR: completion directory not found at $COMPLETIONS_DIR" >&2
  exit 1
fi

mkdir -p "$SDK_DIR"
cd "$SDK_DIR"

if ! grep -qE '^\s*src-link\s+xp2p\s+' feeds.conf.default 2>/dev/null; then
  echo "src-link xp2p $FEED_PATH" >> feeds.conf.default
fi

rm -rf feeds/xp2p package/feeds/xp2p 2>/dev/null || true

echo "==> [$TARGET] Updating feed"
./scripts/feeds update xp2p
./scripts/feeds install xp2p

SDK_MAKEFILE="package/feeds/xp2p/xp2p/Makefile"
REPO_MAKEFILE="$FEED_PACKAGE_PATH/Makefile"
if [ -f "$SDK_MAKEFILE" ] && ! cmp -s "$SDK_MAKEFILE" "$REPO_MAKEFILE"; then
  echo "WARNING: SDK Makefile differs from repository copy ($SDK_MAKEFILE)" >&2
fi

if [ -n "$DIFFCONFIG" ]; then
  if [ -f "$DIFFCONFIG" ]; then
    echo "==> [$TARGET] Applying diffconfig from $DIFFCONFIG"
    cp "$DIFFCONFIG" .config
  else
    echo "WARNING: diffconfig $DIFFCONFIG not found, skipping" >&2
  fi
fi

echo "==> [$TARGET] Running defconfig"
make defconfig

if ! grep -q '^CONFIG_PACKAGE_xp2p=y' .config; then
  echo "CONFIG_PACKAGE_xp2p=y" >> .config
  make defconfig
fi

if [ -n "$DIFFCONFIG_OUT" ]; then
  echo "==> [$TARGET] Writing diffconfig to $DIFFCONFIG_OUT"
  ./scripts/diffconfig.sh > "$DIFFCONFIG_OUT"
fi

echo "==> [$TARGET] Building xp2p ipk"
XP2P_PREBUILT="$XP2P_BIN" \
XP2P_XRAY="$XRAY_BIN" \
XP2P_COMPLETIONS="$COMPLETIONS_DIR" \
  make package/xp2p/clean V=sc >/dev/null 2>&1 || true
XP2P_PREBUILT="$XP2P_BIN" \
XP2P_XRAY="$XRAY_BIN" \
XP2P_COMPLETIONS="$COMPLETIONS_DIR" \
  make package/xp2p/compile V=sc

echo "==> [$TARGET] Collecting artefact"
IPK_PATH=$(find "$SDK_DIR/bin/packages" -type f -name "xp2p_*.ipk" -print | sort | tail -n1 || true)
if [ -z "$IPK_PATH" ]; then
  echo "ERROR: xp2p ipk not found in $SDK_DIR/bin/packages" >&2
  exit 1
fi

ARCH_DIR=$(tar -xzOf "$IPK_PATH" ./control.tar.gz | tar -xzOf - ./control | awk -F': ' '/^Architecture:/ {print $2; exit}')
if [ -z "$ARCH_DIR" ]; then
  echo "ERROR: unable to determine architecture for $IPK_PATH" >&2
  exit 1
fi

DEST_DIR="$REPO_ROOT/$RELEASE_VERSION/$ARCH_DIR"
mkdir -p "$DEST_DIR"
cp "$IPK_PATH" "$DEST_DIR/"

echo "==> [$TARGET] Updating feed index at $DEST_DIR"
"$PROJECT_ROOT/scripts/build/make_openwrt_packages.sh" --path "$DEST_DIR"

echo "Build complete: $(basename "$IPK_PATH")"
echo "Stored under: $DEST_DIR"
