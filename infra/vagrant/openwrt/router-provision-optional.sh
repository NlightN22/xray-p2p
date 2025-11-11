# Optional: section
# disable ipv6
grep -q '^net.ipv6.conf.all.disable_ipv6=1' /etc/sysctl.conf || cat <<'EOF' >> /etc/sysctl.conf
net.ipv6.conf.all.disable_ipv6=1
net.ipv6.conf.default.disable_ipv6=1
net.ipv6.conf.lo.disable_ipv6=1
EOF
sysctl -p /etc/sysctl.conf

uci set network.wan6.proto='none'
uci commit network
/etc/init.d/network restart

# install dig
opkg update
opkg install bind-dig

# delete sing-box
opkg update
opkg remove sing-box || true
rm -f /usr/bin/sing-box || true
rm -rf /etc/sing-box || true
rm -f /var/log/sing-box.log || true