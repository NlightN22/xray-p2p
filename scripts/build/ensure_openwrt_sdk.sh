#!/bin/sh
set -eu

DEFAULT_OPENWRT_VERSION="23.05.3"
OPENWRT_VERSION=${OPENWRT_VERSION:-$DEFAULT_OPENWRT_VERSION}
OPENWRT_MIRROR=${OPENWRT_MIRROR:-"https://downloads.openwrt.org/releases"}
OPENWRT_SDK_BASE=${OPENWRT_SDK_BASE:-"$HOME"}
METADATA_FILE=".xp2p-openwrt-version"

usage() {
  cat <<'EOF'
Usage: scripts/build/ensure_openwrt_sdk.sh [options] [identifier ...]

Ensures that each requested OpenWrt SDK (23.05.x by default) is downloaded
under ~/openwrt-sdk-<identifier>. Supported identifiers:
  - linux-amd64
  - linux-386
  - linux-arm64
  - linux-armhf
  - linux-mipsle-softfloat
  - linux-mips64le

Set OPENWRT_VERSION/OPENWRT_MIRROR/OPENWRT_SDK_BASE when customization is needed.
Options:
  -r, --release <ver>   OpenWrt release version (default: 23.05.3)
  -h, --help            Show this message
EOF
}

download_file() {
  url=$1
  dest=$2
  if command -v curl >/dev/null 2>&1; then
    if curl -fL "$url" -o "$dest"; then
      return 0
    fi
    return $?
  fi
  if wget -qO "$dest" "$url"; then
    return 0
  fi
  return $?
}

resolve_identifier() {
  identifier=$1
  case "$identifier" in
    linux-amd64)
      TARGET="x86"
      SUBTARGET="64"
      FEED_SEGMENT="x86/64"
      TARBALL_SUFFIXES="x86-64_gcc-13.3.0_musl.Linux-x86_64.tar.zst x86-64_gcc-12.3.0_musl.Linux-x86_64.tar.xz"
      ;;
    linux-386)
      TARGET="x86"
      SUBTARGET="generic"
      FEED_SEGMENT="x86/generic"
      TARBALL_SUFFIXES="x86-generic_gcc-13.3.0_musl.Linux-x86_64.tar.zst x86-generic_gcc-12.3.0_musl.Linux-x86_64.tar.xz"
      ;;
    linux-arm64)
      TARGET="armsr"
      SUBTARGET="armv8"
      FEED_SEGMENT="armsr/armv8"
      TARBALL_SUFFIXES="armsr-armv8_gcc-13.3.0_musl.Linux-x86_64.tar.zst armsr-armv8_gcc-12.3.0_musl.Linux-x86_64.tar.xz"
      ;;
    linux-armhf)
      TARGET="armsr"
      SUBTARGET="armv7"
      FEED_SEGMENT="armsr/armv7"
      TARBALL_SUFFIXES="armsr-armv7_gcc-13.3.0_musl_eabi.Linux-x86_64.tar.zst armsr-armv7_gcc-12.3.0_musl_eabi.Linux-x86_64.tar.xz"
      ;;
    linux-mipsle-softfloat)
      TARGET="ramips"
      SUBTARGET="mt7621"
      FEED_SEGMENT="ramips/mt7621"
      TARBALL_SUFFIXES="ramips-mt7621_gcc-13.3.0_musl.Linux-x86_64.tar.zst ramips-mt7621_gcc-12.3.0_musl.Linux-x86_64.tar.xz"
      ;;
    linux-mips64le)
      TARGET="malta"
      SUBTARGET="be"
      FEED_SEGMENT="malta/be"
      TARBALL_SUFFIXES="malta-be_gcc-13.3.0_musl.Linux-x86_64.tar.zst malta-be_gcc-12.3.0_musl.Linux-x86_64.tar.xz"
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

  sdk_dir="${OPENWRT_SDK_BASE%/}/openwrt-sdk-${OPENWRT_VERSION}-${identifier}"
  version_token="${OPENWRT_VERSION}-${TARGET}-${SUBTARGET}"
  tarball=""

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

  archive=""
  for suffix in $TARBALL_SUFFIXES; do
    candidate="openwrt-sdk-${OPENWRT_VERSION}-${suffix}"
    url="${OPENWRT_MIRROR}/${OPENWRT_VERSION}/targets/${FEED_SEGMENT}/${candidate}"
    archive="$tmp_dir/${candidate##*/}"
    echo "==> [$identifier] Downloading $url"
    if download_file "$url" "$archive"; then
      tarball="$candidate"
      break
    fi
    echo "==> [$identifier] Failed to download $candidate, trying fallback..." >&2
    archive=""
    tarball=""
  done
  if [ -z "$tarball" ] || [ -z "$archive" ]; then
    echo "ERROR: Unable to download OpenWrt SDK for $identifier (release $OPENWRT_VERSION)" >&2
    exit 1
  fi

  extracted_dir=$(tar -tf "$archive" | head -n 1 | cut -d/ -f1)
  tar -C "$tmp_dir" -xf "$archive"
  mkdir -p "$(dirname "$sdk_dir")"
  mv "$tmp_dir/$extracted_dir" "$sdk_dir"
  echo "$version_token" > "$sdk_dir/$METADATA_FILE"
  rm -rf "$tmp_dir"
  trap - EXIT

  echo "==> [$identifier] SDK ready at $sdk_dir"
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    -r|--release)
      if [ "${2:-}" = "" ]; then
        echo "ERROR: --release requires a version argument" >&2
        exit 1
      fi
      OPENWRT_VERSION="$2"
      shift 2
      ;;
    --release=*)
      OPENWRT_VERSION="${1#*=}"
      shift
      ;;
    --)
      shift
      break
      ;;
    -*)
      echo "ERROR: unknown option '$1'" >&2
      usage
      exit 1
      ;;
    *)
      break
      ;;
  esac
done

if [ "$#" -eq 0 ]; then
  set -- linux-amd64 linux-386 linux-arm64 linux-armhf linux-mipsle-softfloat linux-mips64le
fi

for identifier in "$@"; do
  ensure_sdk "$identifier"
done
