#!/bin/sh
set -eu

protocol="$1"
credential="$2"
server_port="$3"
socks_port="$4"
config_path="/tmp/xp2p-3xui-$protocol.json"
log_path="/tmp/xp2p-3xui-$protocol.log"
certificate="/srv/xray-p2p/tests/fixtures/tls/integration-cert.pem"
pin="$(openssl x509 -in "$certificate" -outform DER | sha256sum | awk '{print $1}')"

if [ "$protocol" = "vless" ]; then
  user="{\"id\":\"$credential\",\"encryption\":\"none\",\"flow\":\"xtls-rprx-vision\"}"
  settings="{\"vnext\":[{\"address\":\"10.62.10.13\",\"port\":$server_port,\"users\":[$user]}]}"
else
  settings="{\"servers\":[{\"address\":\"10.62.10.13\",\"port\":$server_port,\"password\":\"$credential\"}]}"
fi

cat >"$config_path" <<EOF
{
  "log": {"loglevel": "warning"},
  "inbounds": [{"listen": "127.0.0.1", "port": $socks_port, "protocol": "socks", "settings": {"udp": false}}],
  "outbounds": [{
    "tag": "fixture",
    "protocol": "$protocol",
    "settings": $settings,
    "streamSettings": {"network": "tcp", "security": "tls", "tlsSettings": {"serverName": "xp2p-integration.local", "pinnedPeerCertSha256": "$pin"}}
  }]
}
EOF

/etc/xp2p/bin/xray run -config "$config_path" >"$log_path" 2>&1 &
xray_pid="$!"
trap 'kill "$xray_pid" 2>/dev/null || true; wait "$xray_pid" 2>/dev/null || true; rm -f "$config_path" "$log_path"' EXIT

attempt=0
until curl --fail --silent --max-time 5 --socks5-hostname "127.0.0.1:$socks_port" --output /dev/null https://example.com/; do
  attempt=$((attempt + 1))
  [ "$attempt" -lt 20 ] || { sed -E 's/(password|id)\":\"[^\"]+/\1\":\"[REDACTED]/g' "$log_path" >&2; exit 1; }
  sleep 1
done
