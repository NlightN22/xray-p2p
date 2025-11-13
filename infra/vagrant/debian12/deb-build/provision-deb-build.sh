#!/bin/bash
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive

APT_PACKAGES="
  build-essential
  ca-certificates
  curl
  debhelper
  fpm
  git
  lintian
  pkg-config
  rpm
  rsync
  ruby
  ruby-dev
  unzip
"

apt-get update -y
apt-get install -y --no-install-recommends $APT_PACKAGES
apt-get clean
rm -rf /var/lib/apt/lists/*

GO_VERSION=${GO_VERSION:-"1.23.3"}
GO_ARCHIVE="go${GO_VERSION}.linux-amd64.tar.gz"
TMP_GO_DIR=$(mktemp -d)

cleanup_go() {
  rm -rf "$TMP_GO_DIR"
}
trap cleanup_go EXIT

if ! command -v go >/dev/null 2>&1 || ! go version | grep -q "go${GO_VERSION}"; then
  echo "Installing Go ${GO_VERSION}"
  curl -fsSL "https://go.dev/dl/${GO_ARCHIVE}" -o "$TMP_GO_DIR/$GO_ARCHIVE"
  rm -rf /usr/local/go
  tar -C /usr/local -xzf "$TMP_GO_DIR/$GO_ARCHIVE"
fi

install -m 0755 -d /usr/local/bin
ln -sf /usr/local/go/bin/go /usr/local/bin/go
ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt

cat >/etc/profile.d/go-path.sh <<'EOF'
export PATH=/usr/local/go/bin:$PATH
EOF

PROJECT_ROOT="/srv/xray-p2p"
VAGRANT_HOME="/home/vagrant"
SOURCE_DIR="${VAGRANT_HOME}/xray-p2p"

if [ -d "$PROJECT_ROOT" ]; then
  install -d -m 0755 "$SOURCE_DIR"
  rsync -a --delete "$PROJECT_ROOT/" "$SOURCE_DIR/"
  chown -R vagrant:vagrant "$SOURCE_DIR"
  echo "xp2p sources synced to ${SOURCE_DIR}"
else
  echo "Warning: ${PROJECT_ROOT} is missing; synced folder did not mount?"
fi
