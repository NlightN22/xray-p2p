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
SOURCE_OVERRIDE=${XP2P_STAGE_SOURCE:-1}
SOURCE_ARCHIVE_NAME=""
VERSION_FILE=${XP2P_VERSION_FILE:-"$PROJECT_ROOT/go/internal/version/version.go"}
PACKAGE_MAKEFILE="$FEED_PATH/packages/utils/xp2p/Makefile"

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
  echo "==> [$identifier] Downloading SDK from $url" >&2
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
  if [ ! -f "$sdk_dir/feeds.conf.default" ]; then
    touch "$sdk_dir/feeds.conf.default"
  fi
  if ! grep -qs "^src-link xp2p " "$sdk_dir/feeds.conf.default"; then
    echo "src-link xp2p $FEED_PATH" >> "$sdk_dir/feeds.conf.default"
  fi
}

resolve_pkg_version() {
  if [ -n "$SOURCE_ARCHIVE_NAME" ]; then
    return
  fi
  sync_pkg_version
  local version
  version=$(awk -F':=' '
    $1 ~ /^PKG_VERSION/ {
      gsub(/[[:space:]]/, "", $2);
      print $2;
      exit
    }' "$PACKAGE_MAKEFILE")
  if [ -z "$version" ]; then
    echo "Unable to resolve PKG_VERSION from xp2p Makefile." >&2
    exit 1
  fi
  SOURCE_ARCHIVE_NAME="xp2p-${version}.tar.xz"
}

sync_pkg_version() {
  if [ ! -f "$VERSION_FILE" ] || [ ! -f "$PACKAGE_MAKEFILE" ]; then
    return
  fi

  local version
  version=$(awk -F'"' '
    /var[[:space:]]+current/ {
      print $2;
      exit
    }' "$VERSION_FILE")

  if [ -z "$version" ]; then
    echo "WARNING: unable to read version from $VERSION_FILE" >&2
    return
  fi

  local current
  current=$(awk -F':=' '
    $1 ~ /^PKG_VERSION/ {
      gsub(/[[:space:]]/, "", $2);
      print $2;
      exit
    }' "$PACKAGE_MAKEFILE")

  if [ "$current" = "$version" ]; then
    return
  fi

  echo "==> Syncing PKG_VERSION in $(basename "$PACKAGE_MAKEFILE") to $version"
  tmp_file=$(mktemp)
  awk -v ver="$version" '
    /^PKG_VERSION:=/ { print "PKG_VERSION:="ver; next }
    { print }
  ' "$PACKAGE_MAKEFILE" > "$tmp_file"
  mv "$tmp_file" "$PACKAGE_MAKEFILE"
}

stage_local_source() {
  local sdk_dir=$1
  if [ "$SOURCE_OVERRIDE" != "1" ]; then
    return
  fi

  resolve_pkg_version

  local archive="$sdk_dir/dl/$SOURCE_ARCHIVE_NAME"
  local repo_root=${XP2P_SOURCE_DIR:-$PROJECT_ROOT}
  local tmp_dir
  tmp_dir=$(mktemp -d)
  local version_dir=${SOURCE_ARCHIVE_NAME%.tar.xz}
  local staging_dir="$tmp_dir/$version_dir"

  mkdir -p "$staging_dir"
  rsync -a --delete \
    --exclude '.git' \
    --exclude '.github' \
    --exclude 'build' \
    --exclude '.vagrant' \
    "$repo_root/" "$staging_dir/" >/dev/null

  mkdir -p "$sdk_dir/dl"
  echo "==> Staging local sources into $archive"
  (
    cd "$tmp_dir"
    tar --numeric-owner --owner=0 --group=0 --mode=a-s -cf - "$version_dir" \
      | xz -zc -7e > "$archive".tmp
  )
  mv "$archive".tmp "$archive"
  rm -rf "$tmp_dir"
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
  stage_local_source "$sdk_dir"

  (
    cd "$sdk_dir"
    ./scripts/feeds update -a
    ./scripts/feeds install golang/host
    ./scripts/feeds install xp2p

    if [ "$KEEP_CONFIG" != "1" ]; then
      prepare_config "$sdk_dir" "$target" "$subtarget" "$profile"
    fi

    make defconfig
    CGO_ENABLED=0 make "$BUILD_TARGET" V=sc
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
linux-arm64|armsr|armv8|Generic|armsr/armv8|armsr-armv8_gcc-12.3.0_musl.Linux-x86_64.tar.xz
linux-mipsle-softfloat|ramips|mt7621|Default|ramips/mt7621|ramips-mt7621_gcc-12.3.0_musl.Linux-x86_64.tar.xz
EOF

if [ "$built_any" -eq 0 ]; then
  echo "ERROR: no targets matched XP2P_TARGETS='$TARGET_FILTER'" >&2
  exit 1
fi

echo "Build artifacts are available under $BUILD_OUTPUT_ROOT"
