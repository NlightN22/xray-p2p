#!/bin/ash

MARKER="/etc/router_provisioned"
MARKER_VERSION="v3"

if [ -f "$MARKER" ]; then
  if grep -qx "$MARKER_VERSION" "$MARKER"; then
    echo "Router provisioning already completed; skipping."
    exit 0
  fi
  rm -f "$MARKER"
fi

opkg remove coreutils-nohup coreutils-timeout >/dev/null 2>&1 || true

TUNNEL_IP=$1
LAN_COUNT=$2

uci batch <<EOF
  set network.tunnel=interface
  set network.tunnel.device='eth2'
  set network.tunnel.proto='static'
  set network.tunnel.ipaddr='${TUNNEL_IP}'
  set network.tunnel.netmask='255.255.255.0'
EOF
uci commit network
/etc/init.d/network restart

uci set network.lan.proto='static'
uci set network.lan.ipaddr="10.0.10${LAN_COUNT}.1"
uci set network.lan.netmask='255.255.255.0'
uci commit network
/etc/init.d/network restart

uci set dhcp.lan.ignore='0'
uci set dhcp.lan.start='100'
uci set dhcp.lan.limit='50'
uci set dhcp.lan.leasetime='15m'
uci commit dhcp
/etc/init.d/dnsmasq restart

uci set firewall.@forwarding[0].dest='wan'

zone=$(uci add firewall zone)
uci set firewall.$zone.name='tun'
uci set firewall.$zone.network='tunnel'
uci set firewall.$zone.input='ACCEPT'
uci set firewall.$zone.output='ACCEPT'
uci set firewall.$zone.forward='REJECT'

uci commit firewall
/etc/init.d/firewall restart

echo "$MARKER_VERSION" > "$MARKER"
