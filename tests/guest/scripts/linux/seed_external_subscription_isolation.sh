#!/bin/sh
set -eu

desired="${XP2P_CONFIG_ROOT:-/etc/xp2p}/xp2p-client.toml"
[ -f "$desired" ] || { echo "client Desired is missing" >&2; exit 1; }

cat >>"$desired" <<'EOF'

[[client.endpoints]]
profile = "trojan-tls"
protocol = "trojan"
transport = "tcp"
security = "none"
hostname = "manual-isolation.example"
tag = "manual-isolation"
address = "127.0.0.1"
port = 9
user = "manual-isolation"
password = "manual-isolation-secret"
server_name = ""
disabled = true

[[client.redirects]]
cidr = "192.0.2.0/24"
outbound_tag = "manual-isolation"
disabled = true

[[client.forwards]]
listen_address = "127.0.0.1"
listen_port = 18097
target_host = "192.0.2.10"
target_port = 80
protocol = "tcp"
tag = "manual-isolation-forward"
remark = "manual isolation forward"

[client.reverse.manual-isolation]
channel_id = "manual-isolation-channel"
user_id = "manual-isolation"
host = "192.0.2.20"
tag = "manual-isolation-reverse"
domain = "manual-isolation.example"
endpoint_tag = "manual-isolation"
disabled = true
EOF
