#!/bin/sh
set -eu

operation="${1:-}"
base_url="http://127.0.0.1:2053"
cookie_file="$(mktemp)"
trap 'rm -f "$cookie_file"' EXIT

login_response="$(curl --fail --silent --cookie-jar "$cookie_file" --data 'username=admin&password=admin' "$base_url/login")"
printf '%s' "$login_response" | grep -q '"success":true' || { echo "3x-ui login failed" >&2; exit 1; }

list_inbounds() {
  curl --fail --silent --cookie "$cookie_file" "$base_url/panel/api/inbounds/list"
}

update_inbound() {
  remark="$1"
  protocol="$2"
  item="$(list_inbounds | jq -c --arg remark "$remark" '.obj[] | select(.remark == $remark)')"
  [ -n "$item" ] || { echo "3x-ui fixture inbound is missing" >&2; exit 1; }
  id="$(printf '%s' "$item" | jq -r '.id')"
  settings="$(printf '%s' "$item" | jq -c -r --arg protocol "$protocol" '.settings | fromjson | if $protocol == "trojan" then .clients[0].password = "rotated-trojan-password" else .clients[0].id = "8b1a9953-c461-4c0f-8c8f-7e6f40c6f0ad" end | tojson')"
  response="$(curl --fail --silent --cookie "$cookie_file" \
    --data-urlencode "remark=$(printf '%s' "$item" | jq -r '.remark')" \
    --data-urlencode "enable=$(printf '%s' "$item" | jq -r '.enable')" \
    --data-urlencode "listen=$(printf '%s' "$item" | jq -r '.listen')" \
    --data-urlencode "port=$(printf '%s' "$item" | jq -r '.port')" \
    --data-urlencode "protocol=$(printf '%s' "$item" | jq -r '.protocol')" \
    --data-urlencode "settings=$settings" \
    --data-urlencode "streamSettings=$(printf '%s' "$item" | jq -c -r '.streamSettings | fromjson | tojson')" \
    --data-urlencode "sniffing=$(printf '%s' "$item" | jq -c -r '.sniffing | fromjson | tojson')" \
    "$base_url/panel/api/inbounds/update/$id")"
  printf '%s' "$response" | grep -q '"success":true' || { echo "3x-ui fixture update failed" >&2; exit 1; }
}

case "$operation" in
  rotate-credentials)
    update_inbound "xp2p-trojan" "trojan"
    update_inbound "xp2p-vless" "vless"
    ;;
  remove-vless)
    item="$(list_inbounds | jq -c '.obj[] | select(.remark == "xp2p-vless")')"
    [ -n "$item" ] || { echo "3x-ui fixture inbound is missing" >&2; exit 1; }
    id="$(printf '%s' "$item" | jq -r '.id')"
    response="$(curl --fail --silent --cookie "$cookie_file" -X POST "$base_url/panel/api/inbounds/del/$id")"
    printf '%s' "$response" | grep -q '"success":true' || { echo "3x-ui fixture removal failed" >&2; exit 1; }
    ;;
  *)
    echo "unsupported fixture mutation" >&2
    exit 2
    ;;
esac

attempt=0
until curl --fail --silent "http://127.0.0.1:2096/sub/xp2pfixture2811" >/dev/null; do
  attempt=$((attempt + 1))
  [ "$attempt" -lt 30 ] || { echo "3x-ui subscription did not become ready" >&2; exit 1; }
  sleep 1
done
