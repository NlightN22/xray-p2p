#!/bin/bash
set -euo pipefail

REALM='xp2p-identity-test'
CLIENT_ID='xp2p-directory'
CLIENT_SECRET='integration-directory-secret'
ADMIN_USERNAME='integration-admin'
ADMIN_PASSWORD='integration-admin-password'
WORK_DIR='/srv/xray-p2p/build/identity'
COMPOSE_FILE='/srv/xray-p2p/tests/guest/fixtures/identity/keycloak-compose.yaml'
WORK_COMPOSE_FILE="$WORK_DIR/compose.yaml"
REALM_FILE='/srv/xray-p2p/tests/guest/fixtures/identity/keycloak-realm.json'
LOCAL_URL='http://127.0.0.1:8080'

usage() { echo "usage: $0 {prepare|reset|cleanup|token|list-users|list-groups|list-group-members|set-membership|add-user|remove-user|add-group|remove-group} [args...]" >&2; exit 2; }
compose() {
  if docker compose version >/dev/null 2>&1; then
    docker compose --project-name xp2p-identity --project-directory "$WORK_DIR" -f "$WORK_COMPOSE_FILE" "$@"
  else
    docker-compose --project-name xp2p-identity --project-directory "$WORK_DIR" -f "$WORK_COMPOSE_FILE" "$@"
  fi
}
dump_failure() { mkdir -p "$WORK_DIR/logs"; compose logs --no-color >"$WORK_DIR/logs/keycloak.log" 2>&1 || true; }
trap 'dump_failure' ERR

admin_token() {
  curl -fsS -X POST "$LOCAL_URL/realms/master/protocol/openid-connect/token" \
    -d 'grant_type=password' -d 'client_id=admin-cli' -d "username=$ADMIN_USERNAME" -d "password=$ADMIN_PASSWORD" | json_value access_token
}
directory_token() {
  curl -fsS -X POST "${KEYCLOAK_URL:-$LOCAL_URL}/realms/$REALM/protocol/openid-connect/token" \
    -d 'grant_type=client_credentials' -d "client_id=$CLIENT_ID" -d "client_secret=$CLIENT_SECRET" | json_value access_token
}
json_value() { python3 -c 'import json, sys; print(json.load(sys.stdin)[sys.argv[1]])' "$1"; }
api() { local token=$1 method=$2 path=$3; shift 3; curl -fsS -X "$method" -H "Authorization: Bearer $token" -H 'Content-Type: application/json' "$LOCAL_URL/admin/realms/$REALM$path" "$@"; }
directory_api() { local token=$1 path=$2; curl -fsS -H "Authorization: Bearer $token" "${KEYCLOAK_URL:-$LOCAL_URL}/admin/realms/$REALM$path"; }
user_id() { api "$1" GET "/users?username=$2&exact=true" | jq -r '.[0].id // empty'; }
group_id() { api "$1" GET "/groups?search=$2&exact=true" | jq -r '.[] | select(.name == "'"$2"'") | .id'; }
directory_group_id() { directory_api "$1" "/groups?search=$2&exact=true" | python3 -c 'import json, sys; name=sys.argv[1]; print(next(group["id"] for group in json.load(sys.stdin) if group["name"] == name))' "$2"; }

prepare() {
  install -d -m 0755 "$WORK_DIR/realm" "$WORK_DIR/logs"
  cp "$REALM_FILE" "$WORK_DIR/realm/$REALM-realm.json"
  cp "$COMPOSE_FILE" "$WORK_COMPOSE_FILE"
  KC_BOOTSTRAP_ADMIN_USERNAME="$ADMIN_USERNAME" KC_BOOTSTRAP_ADMIN_PASSWORD="$ADMIN_PASSWORD" compose up -d
  for _ in $(seq 1 60); do curl -fsS "$LOCAL_URL/realms/master/.well-known/openid-configuration" >/dev/null 2>&1 && break; sleep 1; done
  curl -fsS "$LOCAL_URL/realms/master/.well-known/openid-configuration" >/dev/null
  directory_token >/dev/null
}
reset() { cleanup; prepare; }
cleanup() { compose down --volumes --remove-orphans >/dev/null 2>&1 || true; rm -rf "$WORK_DIR"; }

case "${1:-}" in
  prepare) prepare ;;
  reset) reset ;;
  cleanup) cleanup ;;
  token) directory_token ;;
  list-users) directory_api "$(directory_token)" '/users?briefRepresentation=true' ;;
  list-groups) directory_api "$(directory_token)" '/groups?briefRepresentation=true' ;;
  list-group-members) [ "$#" = 2 ] || usage; token=$(directory_token); group=$(directory_group_id "$token" "$2"); directory_api "$token" "/groups/$group/members" ;;
  set-membership)
    [ "$#" = 4 ] || usage; token=$(admin_token); user=$(user_id "$token" "$2"); group=$(group_id "$token" "$3"); [ -n "$user" ] && [ -n "$group" ] || { echo 'User or group not found' >&2; exit 1; }
    case "$4" in present) api "$token" PUT "/users/$user/groups/$group" >/dev/null ;; absent) api "$token" DELETE "/users/$user/groups/$group" >/dev/null ;; *) usage ;; esac ;;
  add-user) [ "$#" = 6 ] || usage; token=$(admin_token); api "$token" POST '/users' --data "{\"id\":\"$2\",\"username\":\"$3\",\"firstName\":\"$4\",\"lastName\":\"$5\",\"email\":\"$6\",\"enabled\":true}" >/dev/null ;;
  remove-user) [ "$#" = 2 ] || usage; token=$(admin_token); user=$(user_id "$token" "$2"); [ -z "$user" ] || api "$token" DELETE "/users/$user" >/dev/null ;;
  add-group) [ "$#" = 3 ] || usage; api "$(admin_token)" POST '/groups' --data "{\"id\":\"$2\",\"name\":\"$3\"}" >/dev/null ;;
  remove-group) [ "$#" = 2 ] || usage; token=$(admin_token); group=$(group_id "$token" "$2"); [ -z "$group" ] || api "$token" DELETE "/groups/$group" >/dev/null ;;
  *) usage ;;
esac
