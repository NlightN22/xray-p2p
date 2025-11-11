#!/bin/sh
set -eu

script_dir=$(cd "$(dirname "$0")" && pwd)
PROJECT_ROOT=${XP2P_PROJECT_ROOT:-$(cd "$script_dir/../../.." && pwd)}
FEED_PATH=${XP2P_FEED_PATH:-"$PROJECT_ROOT/openwrt/feed"}
BUILD_TARGET=${XP2P_BUILD_TARGET:-"package/xp2p/compile"}
BUILD_OUTPUT_ROOT=${XP2P_BUILD_ROOT:-"$PROJECT_ROOT/build/openwrt"}
SDK_BASE_DIR=${XP2P_SDK_BASE:-"$HOME"}
OPENWRT_VERSION=${XP2P_OPENWRT_VERSION:-"23.05.3"}
OPENWRT_MIRROR=${XP2P_OPENWRT_MIRROR:-"https://downloads.openwrt.org/releases"}
TARGET_FILTER=${XP2P_TARGETS:-"all"}
KEEP_CONFIG=${XP2P_KEEP_CONFIG:-0}

mkdir -p "$BUILD_OUTPUT_ROOT"

lower() {
  printf "%s" "$1" | tr '[:upper:]' '[:lower:]'
}

normalize_filter() {
  local normalized
  normalized=$(printf "%s" "$TARGET_FILTER" | tr ',\t\r ' '\n' | sed '/^$/d' | tr '[:upper:]' '[:lower:]')
  if [ -z "$normalized" ]; then
    TARGET_FILTER_LIST=" all "
  else
    TARGET_FILTER_LIST=" $(printf "%s" "$normalized" | tr '\n' ' ') "
  fi
}

should_build() {
  local ident
  ident=$(lower "$1")
  case "$TARGET_FILTER_LIST" in
    *" all "*|*" $ident "*) return 0 ;;
    *) return 1 ;;
  esac
}

download_file() {
  local url=$1
  local dest=$2
  if command -v curl >/dev/null 2>&1; then
    curl -fL "$url" -o "$dest"
  else
    wget -qO "$dest" "$url"
  fi
}

ensure_sdk() {
  local identifier=$1
  local feed_segment=$2
  local tarball_suffix=$3

  local default_dir="$SDK_BASE_DIR/openwrt-sdk-$identifier"
  local legacy_dir="$SDK_BASE_DIR/openwrt-sdk"
  local sdk_dir="$default_dir"
  if [ "$identifier" = "linux-amd64" ] && [ ! -d "$sdk_dir" ] && [ -d "$legacy_dir" ]; then
    sdk_dir="$legacy_dir"
  fi

  if [ -d "$sdk_dir" ] && [ -n "$(ls -A "$sdk_dir" 2>/dev/null)" ]; then
    printf "%s" "$sdk_dir"
    return
  fi

  mkdir -p "$SDK_BASE_DIR"
  local tmp_dir
  tmp_dir=$(mktemp -d)
  local tarball="openwrt-sdk-${OPENWRT_VERSION}-${tarball_suffix}"
  local url="${OPENWRT_MIRROR}/${OPENWRT_VERSION}/targets/${feed_segment}/${tarball}"
  echo "==> [$identifier] Downloading SDK from $url"
  download_file "$url" "$tmp_dir/sdk.tar.xz"
  local extracted
  extracted=$(tar -tf "$tmp_dir/sdk.tar.xz" | head -n 1 | cut -d/ -f1)
  tar -C "$tmp_dir" -xf "$tmp_dir/sdk.tar.xz"
  rm -rf "$sdk_dir"
  mv "$tmp_dir/$extracted" "$sdk_dir"
  rm -rf "$tmp_dir"
  printf "%s" "$sdk_dir"
}

prepare_config() {
  local sdk_dir=$1
  local target=$2
  local subtarget=$3
  local profile=$4

  sanitize() {
    printf "%s" "$1" | tr '/-' '__'
  }

  local target_token subtarget_token profile_token
  target_token=$(sanitize "$target")
  subtarget_token=$(sanitize "$subtarget")
  profile_token=$(sanitize "$profile")

  {
    printf 'CONFIG_TARGET_%s=y\n' "$target_token"
    printf 'CONFIG_TARGET_%s_%s=y\n' "$target_token" "$subtarget_token"
    if [ -n "$profile" ]; then
      printf 'CONFIG_TARGET_%s_%s_%s=y\n' "$target_token" "$subtarget_token" "$profile_token"
    fi
    printf 'CONFIG_PACKAGE_xp2p=y\n'
  } > "$sdk_dir/.config"
}

ensure_feed_link() {
  local sdk_dir=$1
  if ! grep -qs "^src-link xp2p " "$sdk_dir/feeds.conf.default"; then
    echo "src-link xp2p $FEED_PATH" >> "$sdk_dir/feeds.conf.default"
  fi
}

find_recent_ipk() {
  local sdk_dir=$1
  find "$sdk_dir/bin/packages" -type f -name "xp2p_*.ipk" -printf '%T@ %p\n' 2>/dev/null \
    | sort -nr \
    | head -n 1 \
    | cut -d' ' -f2-
}

build_for_target() {
  local identifier=$1
  local target=$2
  local subtarget=$3
  local profile=$4
  local feed_segment=$5
  local tarball_suffix=$6

  local sdk_dir
  sdk_dir=$(ensure_sdk "$identifier" "$feed_segment" "$tarball_suffix")
  echo "==> [$identifier] Using SDK at $sdk_dir"

  ensure_feed_link "$sdk_dir"

  (
    cd "$sdk_dir"
    ./scripts/feeds update -a
    ./scripts/feeds install golang/host
    ./scripts/feeds install xp2p

    if [ "$KEEP_CONFIG" != "1" ]; then
      prepare_config "$sdk_dir" "$target" "$subtarget" "$profile"
    fi

    make defconfig
    make "$BUILD_TARGET" V=sc
  )

  local pkg
  pkg=$(find_recent_ipk "$sdk_dir")
  if [ -z "$pkg" ]; then
    echo "ERROR: [$identifier] xp2p ipk not found in $sdk_dir/bin/packages" >&2
    exit 1
  fi

  local dest_dir="$BUILD_OUTPUT_ROOT/$identifier"
  mkdir -p "$dest_dir"
  cp "$pkg" "$dest_dir/"
  echo "==> [$identifier] Stored $(basename "$pkg") in $dest_dir"
}

normalize_filter
built_any=0

while IFS='|' read -r identifier target subtarget profile feed_segment tarball_suffix; do
  if [ -z "${identifier:-}" ] || [ "${identifier#\#}" != "$identifier" ]; then
    continue
  fi

  if should_build "$identifier"; then
    built_any=1
    build_for_target "$identifier" "$target" "$subtarget" "$profile" "$feed_segment" "$tarball_suffix"
  fi
done <<'EOF'
# identifier|target|subtarget|profile|feed-segment|tarball-suffix
linux-amd64|x86|64|Generic|x86/64|x86-64_gcc-12.3.0_musl.Linux-x86_64.tar.xz
linux-arm64|armvirt|64|Default|armvirt/64|armvirt-64_gcc-12.3.0_musl.Linux-x86_64.tar.xz
linux-mipsle-softfloat|mipsel|24kc||mipsel/24kc|mipsel-24kc_gcc-12.3.0_musl.Linux-x86_64.tar.xz
EOF

if [ "$built_any" -eq 0 ]; then
  echo "ERROR: no targets matched XP2P_TARGETS='$TARGET_FILTER'" >&2
  exit 1
fi

echo "Build artifacts are available under $BUILD_OUTPUT_ROOT"
