#!/bin/bash
set -euo pipefail

BASE_DN='dc=identity,dc=xp2p,dc=test'
ADMIN_DN="cn=admin,$BASE_DN"
ADMIN_PASSWORD='integration-admin-password'
READER_DN="cn=ldap-reader,ou=service,$BASE_DN"
READER_PASSWORD='integration-reader-password'
WORK_DIR='/srv/xray-p2p/build/identity'
FIXTURE='/srv/xray-p2p/tests/guest/fixtures/identity/base.ldif'

usage() {
  echo "usage: $0 {prepare|reset|add-user|remove-user|add-group|remove-group|set-membership|cleanup}" >&2
  exit 2
}

ldap_admin() {
  ldapmodify -x -H ldap://127.0.0.1:389 -D "$ADMIN_DN" -w "$ADMIN_PASSWORD" "$@"
}

user_id() {
  ldapsearch -LLL -x -H ldap://127.0.0.1:389 -D "$ADMIN_DN" -w "$ADMIN_PASSWORD" \
    -b "ou=people,$BASE_DN" "(uid=$1)" employeeNumber | awk '/^employeeNumber: / {print $2; exit}'
}

group_dn() {
  ldapsearch -LLL -x -H ldap://127.0.0.1:389 -D "$ADMIN_DN" -w "$ADMIN_PASSWORD" \
    -b "ou=groups,$BASE_DN" "(cn=$1)" dn | awk '/^dn: / {sub(/^dn: /, ""); print; exit}'
}

reset_directory() {
  ldapdelete -x -H ldap://127.0.0.1:389 -D "$ADMIN_DN" -w "$ADMIN_PASSWORD" -r "ou=service,$BASE_DN" >/dev/null 2>&1 || true
  ldapdelete -x -H ldap://127.0.0.1:389 -D "$ADMIN_DN" -w "$ADMIN_PASSWORD" -r "ou=people,$BASE_DN" >/dev/null 2>&1 || true
  ldapdelete -x -H ldap://127.0.0.1:389 -D "$ADMIN_DN" -w "$ADMIN_PASSWORD" -r "ou=groups,$BASE_DN" >/dev/null 2>&1 || true
  ldapadd -x -H ldap://127.0.0.1:389 -D "$ADMIN_DN" -w "$ADMIN_PASSWORD" -f "$FIXTURE" >/dev/null
}

configure_access() {
  cat <<EOF | ldapmodify -Y EXTERNAL -H ldapi:/// >/dev/null
dn: olcDatabase={1}mdb,cn=config
changetype: modify
replace: olcAccess
olcAccess: {0}to attrs=userPassword by dn.exact="$ADMIN_DN" manage by self write by anonymous auth by * none
olcAccess: {1}to * by dn.exact="$ADMIN_DN" manage by dn.exact="$READER_DN" read by * none
EOF
}

prepare() {
  install -d -m 0755 "$WORK_DIR"
  systemctl enable --now slapd >/dev/null
  for _ in $(seq 1 20); do
    if ldapsearch -LLL -x -H ldap://127.0.0.1:389 -D "$ADMIN_DN" -w "$ADMIN_PASSWORD" -b "$BASE_DN" -s base dn >/dev/null 2>&1; then
      configure_access
      reset_directory
      return
    fi
    sleep 1
  done
  echo 'LDAP did not become ready' >&2
  exit 1
}

case "${1:-}" in
  prepare) prepare ;;
  reset) reset_directory ;;
  add-user)
    [ "$#" = 5 ] || usage
    cat <<EOF | ldap_admin >/dev/null
dn: employeeNumber=$2,ou=people,$BASE_DN
objectClass: top
objectClass: inetOrgPerson
cn: $3
sn: $3
uid: $4
employeeNumber: $2
mail: $5
displayName: $3
EOF
    ;;
  remove-user)
    [ "$#" = 2 ] || usage
    id=$(user_id "$2")
    [ -n "$id" ] || exit 0
    for dn in $(ldapsearch -LLL -x -H ldap://127.0.0.1:389 -D "$ADMIN_DN" -w "$ADMIN_PASSWORD" -b "ou=groups,$BASE_DN" "(memberUid=$id)" dn | sed -n 's/^dn: //p'); do
      printf 'dn: %s\nchangetype: modify\ndelete: memberUid\nmemberUid: %s\n-\n' "$dn" "$id" | ldap_admin >/dev/null
    done
    ldapdelete -x -H ldap://127.0.0.1:389 -D "$ADMIN_DN" -w "$ADMIN_PASSWORD" "employeeNumber=$id,ou=people,$BASE_DN" >/dev/null
    ;;
  add-group)
    [ "$#" = 3 ] || usage
    cat <<EOF | ldap_admin >/dev/null
dn: gidNumber=$2,ou=groups,$BASE_DN
objectClass: top
objectClass: posixGroup
cn: $3
gidNumber: $2
EOF
    ;;
  remove-group)
    [ "$#" = 2 ] || usage
    dn=$(group_dn "$2")
    [ -z "$dn" ] || ldapdelete -x -H ldap://127.0.0.1:389 -D "$ADMIN_DN" -w "$ADMIN_PASSWORD" "$dn" >/dev/null
    ;;
  set-membership)
    [ "$#" = 4 ] || usage
    id=$(user_id "$2")
    dn=$(group_dn "$3")
    [ -n "$id" ] && [ -n "$dn" ] || { echo 'User or group not found' >&2; exit 1; }
    case "$4" in
      present) printf 'dn: %s\nchangetype: modify\nadd: memberUid\nmemberUid: %s\n-\n' "$dn" "$id" | ldap_admin >/dev/null 2>&1 || true ;;
      absent) printf 'dn: %s\nchangetype: modify\ndelete: memberUid\nmemberUid: %s\n-\n' "$dn" "$id" | ldap_admin >/dev/null 2>&1 || true ;;
      *) usage ;;
    esac
    ;;
  cleanup)
    reset_directory
    rm -rf "$WORK_DIR"
    ;;
  *) usage ;;
esac
