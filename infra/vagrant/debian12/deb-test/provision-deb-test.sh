#!/bin/bash
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive

APT_PACKAGES="
  build-essential
  ca-certificates
  curl
  debhelper
  git
  iperf3
  iptables
  iproute2
  iputils-ping
  lintian
  ldap-utils
  nftables
  openssl
  pkg-config
  procps
  psmisc
  python3
  qemu-user-static
  rpm
  rsync
  ruby
  ruby-dev
  unzip
"

if [ "$(hostname -s)" = "deb-test-c" ]; then
  APT_PACKAGES="$APT_PACKAGES
  docker-compose
  docker.io
  jq
  slapd
  "

  debconf-set-selections <<'EOF'
slapd slapd/no_configuration boolean false
slapd slapd/domain string identity.xp2p.test
slapd shared/organization string XP2P Integration Tests
slapd slapd/password1 password integration-admin-password
slapd slapd/password2 password integration-admin-password
slapd slapd/move_old_database boolean true
slapd slapd/purge_database boolean false
slapd slapd/allow_ldap_v2 boolean false
EOF
fi

apt-get update -y
apt-get install -y --no-install-recommends $APT_PACKAGES
apt-get clean
rm -rf /var/lib/apt/lists/*

FIXTURE_CA_SOURCE="/srv/xray-p2p/tests/fixtures/tls/integration-cert.pem"
FIXTURE_CA_TARGET="/usr/local/share/ca-certificates/xp2p-integration.crt"
if [ -f "$FIXTURE_CA_SOURCE" ]; then
  install -m 0644 "$FIXTURE_CA_SOURCE" "$FIXTURE_CA_TARGET"
  update-ca-certificates
fi

if [ "$(hostname -s)" = "deb-test-c" ]; then
  systemctl enable --now docker
fi

# Ensure binfmt handlers for cross-built debs (arm64/armhf/386)
if command -v update-binfmts >/dev/null 2>&1; then
  update-binfmts --display | grep -q "qemu-aarch64 (enabled)" || update-binfmts --enable qemu-aarch64 || true
  update-binfmts --display | grep -q "qemu-arm (enabled)" || update-binfmts --enable qemu-arm || true
  update-binfmts --display | grep -q "qemu-i386 (enabled)" || update-binfmts --enable qemu-i386 || true
fi

if [ -z "${GO_VERSION:-}" ]; then
  if [ -f /srv/xray-p2p/go.mod ]; then
    GO_VERSION=$(awk '$1 == "toolchain" {print $2; exit}' /srv/xray-p2p/go.mod | sed 's/^go//' | tr -d '\r')
    if [ -z "$GO_VERSION" ]; then
      GO_VERSION=$(awk '$1 == "go" {print $2; exit}' /srv/xray-p2p/go.mod | tr -d '\r')
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
