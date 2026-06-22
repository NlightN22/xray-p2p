#!/bin/bash
set -euo pipefail

[ "$#" -ge 3 ] || { echo "usage: $0 <host> <base-dn> <filter> [attributes...]" >&2; exit 2; }
host=$1
base_dn=$2
filter=$3
shift 3
ldapsearch -LLL -x -H "ldap://$host:389" \
  -D 'cn=ldap-reader,ou=service,dc=identity,dc=xp2p,dc=test' \
  -w 'integration-reader-password' -b "$base_dn" "$filter" "$@"
