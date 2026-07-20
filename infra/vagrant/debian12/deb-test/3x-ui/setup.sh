#!/bin/sh
set -eu

base_url="http://127.0.0.1:2053"
cookie_file="$(mktemp)"
trap 'rm -f "$cookie_file"' EXIT

attempt=0
until curl --fail --silent --output /dev/null "$base_url/"; do
  attempt=$((attempt + 1))
  [ "$attempt" -lt 60 ] || { echo "3x-ui panel did not become ready" >&2; exit 1; }
  sleep 1
done

login_response="$(curl --fail --silent --cookie-jar "$cookie_file" --data 'username=admin&password=admin' "$base_url/login")"
printf '%s' "$login_response" | grep -q '"success":true' || { echo "3x-ui login failed" >&2; exit 1; }

add_inbound() {
  remark="$1"
  port="$2"
  protocol="$3"
  settings="$4"
  response="$(curl --fail --silent --cookie "$cookie_file" \
    --data-urlencode "remark=$remark" \
    --data-urlencode 'enable=true' \
    --data-urlencode 'listen=0.0.0.0' \
    --data-urlencode "port=$port" \
    --data-urlencode "protocol=$protocol" \
    --data-urlencode "settings=$settings" \
    --data-urlencode "streamSettings=$common_stream" \
    --data-urlencode "sniffing=$sniffing" \
    "$base_url/panel/api/inbounds/add")"
  printf '%s' "$response" | grep -q '"success":true' || {
    message="$(printf '%s' "$response" | jq -r '.msg // "unknown error"')"
    printf '3x-ui inbound setup failed: %s\n' "$message" >&2
    exit 1
  }
}

common_stream='{"network":"tcp","security":"tls","tlsSettings":{"serverName":"xp2p-integration.local","certificates":[{"certificateFile":"/etc/x-ui/fixture-cert.pem","keyFile":"/etc/x-ui/fixture-key.pem"}]},"tcpSettings":{"acceptProxyProtocol":false,"header":{"type":"none"}}}'
sniffing='{"enabled":false,"destOverride":[]}'

trojan_settings='{"clients":[{"password":"fixture-trojan-password","email":"xp2p-fixture@example.test","subId":"xp2pfixture2811","enable":true}],"fallbacks":[]}'
vless_settings='{"clients":[{"id":"550e8400-e29b-41d4-a716-446655440000","flow":"xtls-rprx-vision","email":"xp2p-vless-fixture@example.test","subId":"xp2pfixture2811","enable":true}],"decryption":"none","fallbacks":[]}'

add_inbound "xp2p-trojan" "16443" "trojan" "$trojan_settings"
add_inbound "xp2p-vless" "16444" "vless" "$vless_settings"

curl --fail --silent "http://127.0.0.1:2096/sub/xp2pfixture2811" >/dev/null
