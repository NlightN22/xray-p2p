#!/bin/sh
set -eu

export DEBIAN_FRONTEND=noninteractive

APT_PACKAGES="bc build-essential ca-certificates curl file gawk git jq golang-go \
  libelf-dev libncurses5-dev libncursesw5-dev libssl-dev pkg-config python3 \
  python3-distutils rsync subversion tar time unzip wget xz-utils zip zlib1g-dev"

apt-get update -y
apt-get install -y --no-install-recommends $APT_PACKAGES
apt-get clean
rm -rf /var/lib/apt/lists/*

OPENWRT_VERSION=${OPENWRT_VERSION:-"23.05.3"}
OPENWRT_TARGET=${OPENWRT_TARGET:-"x86"}
OPENWRT_SUBTARGET=${OPENWRT_SUBTARGET:-"64"}
OPENWRT_SDK_TARBALL=${OPENWRT_SDK_TARBALL:-"openwrt-sdk-${OPENWRT_VERSION}-${OPENWRT_TARGET}-${OPENWRT_SUBTARGET}_gcc-12.3.0_musl.Linux-x86_64.tar.xz"}
OPENWRT_SDK_URL=${OPENWRT_SDK_URL:-"https://downloads.openwrt.org/releases/${OPENWRT_VERSION}/targets/${OPENWRT_TARGET}/${OPENWRT_SUBTARGET}/${OPENWRT_SDK_TARBALL}"}

SDK_DIR=/home/vagrant/openwrt-sdk

if [ ! -d "$SDK_DIR" ] || [ -z "$(ls -A "$SDK_DIR" 2>/dev/null)" ]; then
  tmp_dir=$(mktemp -d)
  sdk_archive="$tmp_dir/openwrt-sdk.tar.xz"

  echo "Downloading OpenWrt SDK from $OPENWRT_SDK_URL"
  wget -qO "$sdk_archive" "$OPENWRT_SDK_URL"

  extracted_dir=$(tar -tf "$sdk_archive" | head -n 1 | cut -d/ -f1)

  echo "Extracting SDK into $SDK_DIR"
  tar -C /home/vagrant -xf "$sdk_archive"
  rm -rf "$SDK_DIR"
  mv "/home/vagrant/$extracted_dir" "$SDK_DIR"
  chown -R vagrant:vagrant "$SDK_DIR"

  rm -rf "$tmp_dir"
else
  echo "OpenWrt SDK already present at $SDK_DIR; skipping download."
fi
