#!/bin/sh
set -eu

OPENWRT_VERSION=${OPENWRT_VERSION:-"23.05.3"}
OPENWRT_MIRROR=${OPENWRT_MIRROR:-"https://downloads.openwrt.org/releases"}
OPENWRT_SDK_BASE=${OPENWRT_SDK_BASE:-"$HOME"}
METADATA_FILE=".xp2p-openwrt-version"

usage() {
  cat <<'EOF'
Usage: scripts/build/ensure_openwrt_sdk.sh [identifier ...]

Ensures that each requested OpenWrt SDK (23.05.x by default) is downloaded
under ~/openwrt-sdk-<identifier>. Supported identifiers:
  - linux-amd64
  - linux-386
  - linux-arm64
  - linux-armhf
  - linux-mipsle-softfloat

Set OPENWRT_VERSION/OPENWRT_MIRROR/OPENWRT_SDK_BASE when customization is needed.
EOF
}

download_file() {
  url=$1
  dest=$2
  if command -v curl >/dev/null 2>&1; then
    curl -fL "$url" -o "$dest"
  else
    wget -qO "$dest" "$url"
  fi
}

resolve_identifier() {
  identifier=$1
  case "$identifier" in
    linux-amd64)
      TARGET="x86"
      SUBTARGET="64"
      FEED_SEGMENT="x86/64"
      TARBALL_SUFFIX="x86-64_gcc-12.3.0_musl.Linux-x86_64.tar.xz"
      ;;
    linux-386)
      TARGET="x86"
      SUBTARGET="generic"
      FEED_SEGMENT="x86/generic"
      TARBALL_SUFFIX="x86-generic_gcc-12.3.0_musl.Linux-x86_64.tar.xz"
      ;;
    linux-arm64)
      TARGET="armsr"
      SUBTARGET="armv8"
      FEED_SEGMENT="armsr/armv8"
      TARBALL_SUFFIX="armsr-armv8_gcc-12.3.0_musl.Linux-x86_64.tar.xz"
      ;;
    linux-armhf)
      TARGET="armsr"
      SUBTARGET="armv7"
      FEED_SEGMENT="armsr/armv7"
      TARBALL_SUFFIX="armsr-armv7_gcc-12.3.0_musl_eabi.Linux-x86_64.tar.xz"
      ;;
    linux-mipsle-softfloat)
      TARGET="ramips"
      SUBTARGET="mt7621"
      FEED_SEGMENT="ramips/mt7621"
      TARBALL_SUFFIX="ramips-mt7621_gcc-12.3.0_musl.Linux-x86_64.tar.xz"
      ;;
    *)
      echo "ERROR: unsupported identifier '$identifier'" >&2
      exit 1
      ;;
  esac
}

ensure_sdk() {
  identifier=$1
  resolve_identifier "$identifier"

  sdk_dir="${OPENWRT_SDK_BASE%/}/openwrt-sdk-$identifier"
  version_token="${OPENWRT_VERSION}-${TARGET}-${SUBTARGET}"
  tarball="openwrt-sdk-${OPENWRT_VERSION}-${TARBALL_SUFFIX}"
  download_url="${OPENWRT_MIRROR}/${OPENWRT_VERSION}/targets/${FEED_SEGMENT}/${tarball}"

  if [ -d "$sdk_dir" ]; then
    if [ -f "$sdk_dir/$METADATA_FILE" ] && [ "$(cat "$sdk_dir/$METADATA_FILE")" = "$version_token" ]; then
      echo "==> [$identifier] SDK already present at $sdk_dir"
      return
    fi
    echo "==> [$identifier] Removing outdated SDK at $sdk_dir"
    rm -rf "$sdk_dir"
  fi

  tmp_dir=$(mktemp -d)
  trap 'rm -rf "$tmp_dir"' EXIT

  archive="$tmp_dir/sdk.tar.xz"
  echo "==> [$identifier] Downloading $download_url"
  download_file "$download_url" "$archive"

  extracted_dir=$(tar -tf "$archive" | head -n 1 | cut -d/ -f1)
  tar -C "$tmp_dir" -xf "$archive"
  mkdir -p "$(dirname "$sdk_dir")"
  mv "$tmp_dir/$extracted_dir" "$sdk_dir"
  echo "$version_token" > "$sdk_dir/$METADATA_FILE"
  rm -rf "$tmp_dir"
  trap - EXIT

  echo "==> [$identifier] SDK ready at $sdk_dir"
}

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  usage
  exit 0
fi

if [ "$#" -eq 0 ]; then
  set -- linux-amd64 linux-386 linux-arm64 linux-armhf linux-mipsle-softfloat
fi

for identifier in "$@"; do
  ensure_sdk "$identifier"
done
