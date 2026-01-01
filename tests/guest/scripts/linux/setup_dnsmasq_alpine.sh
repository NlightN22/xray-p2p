#!/bin/sh
set -eu

if [ "$#" -lt 1 ]; then
  echo "Usage: setup_dnsmasq_alpine.sh <hostname-or-fqdn>" >&2
  exit 2
fi

host_name="$1"
installer=""

for candidate in \
  "/srv/xray-p2p/build/linux-amd64/dnsmasq-install-alpine.sh" \
  "/tmp/dnsmasq-install-alpine.sh" \
  "/srv/xray-p2p/infra/vagrant/openwrt/dnsmasq-install-alpine.sh" \
  "/srv/xray-p2p/infra/vagrant/openwrt-scripts/dnsmasq-install-alpine.sh"
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

chmod +x "$installer"
exec "$installer" "$host_name"
