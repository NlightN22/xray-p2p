#!/bin/bash
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive

APT_PACKAGES="
  build-essential
  ca-certificates
  curl
  debhelper
  git
  iptables
  lintian
  nftables
  pkg-config
  qemu-user-static
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

# Ensure binfmt handlers for cross-built debs (arm64/armhf/386)
if command -v update-binfmts >/dev/null 2>&1; then
  update-binfmts --display | grep -q "qemu-aarch64 (enabled)" || update-binfmts --enable qemu-aarch64 || true
  update-binfmts --display | grep -q "qemu-arm (enabled)" || update-binfmts --enable qemu-arm || true
  update-binfmts --display | grep -q "qemu-i386 (enabled)" || update-binfmts --enable qemu-i386 || true
fi

if [ -z "${GO_VERSION:-}" ]; then
  if [ -f /srv/xray-p2p/go.mod ]; then
    GO_VERSION=$(awk '$1 == "toolchain" {print $2; exit}' /srv/xray-p2p/go.mod | sed 's/^go//')
    if [ -z "$GO_VERSION" ]; then
      GO_VERSION=$(awk '$1 == "go" {print $2; exit}' /srv/xray-p2p/go.mod)
    fi
  fi
fi
GO_VERSION=${GO_VERSION:-"1.25.7"}
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

if ! command -v fpm >/dev/null 2>&1; then
  echo "Installing fpm via RubyGems"
  gem install --no-document fpm
fi

PROJECT_ROOT="/srv/xray-p2p"
if [ ! -d "$PROJECT_ROOT" ]; then
  echo "Warning: ${PROJECT_ROOT} is missing; synced folder did not mount?"
else
  echo "xp2p sources available at ${PROJECT_ROOT}"
fi
