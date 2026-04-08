#!/bin/sh
set -eu

if [ "$#" -lt 1 ]; then
  echo "Usage: setup_dnsmasq_alpine.sh <hostname-or-fqdn>" >&2
  exit 2
fi

host_name="$1"
installer=""
timeout_cmd=""

if command -v timeout >/dev/null 2>&1; then
  timeout_cmd="timeout ${DNSMASQ_INSTALL_TIMEOUT:-300}"
fi

for candidate in \
  "/srv/xray-p2p/build/linux-amd64/dnsmasq-install-alpine.sh" \
  "/tmp/dnsmasq-install-alpine.sh" \
  "/srv/xray-p2p/infra/vagrant/openwrt/dnsmasq-install-alpine.sh"
do
  if [ -f "$candidate" ]; then
    installer="$candidate"
    break
  fi
done

if [ -z "$installer" ]; then
  echo "dnsmasq installer not found on guest" >&2
  exit 1
fi

echo "Using dnsmasq installer: $installer"
if [ -n "$timeout_cmd" ]; then
  if [ -x "$installer" ]; then
    exec $timeout_cmd "$installer" "$host_name"
  fi
  exec $timeout_cmd sh "$installer" "$host_name"
fi
if [ -x "$installer" ]; then
  exec "$installer" "$host_name"
fi
exec sh "$installer" "$host_name"
