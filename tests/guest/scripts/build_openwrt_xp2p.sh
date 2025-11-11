#!/bin/sh
set -eu

SDK_DIR=${OPENWRT_SDK_DIR:-/home/vagrant/openwrt-sdk}
FEED_PATH=${XP2P_FEED_PATH:-/srv/xray-p2p/openwrt/feed}
BUILD_TARGET=${XP2P_BUILD_TARGET:-"package/xp2p/compile"}

if [ ! -d "$SDK_DIR" ]; then
  echo "OpenWrt SDK directory $SDK_DIR is missing" >&2
  exit 1
fi

if [ ! -d "$FEED_PATH" ]; then
  echo "xp2p feed path $FEED_PATH is missing" >&2
  exit 1
fi

cd "$SDK_DIR"

if ! grep -qs "^src-link xp2p " feeds.conf.default; then
  echo "src-link xp2p $FEED_PATH" >> feeds.conf.default
fi

./scripts/feeds update xp2p
./scripts/feeds install xp2p
make "$BUILD_TARGET" V=sc
