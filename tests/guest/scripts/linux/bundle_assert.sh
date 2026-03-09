#!/bin/bash
set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "Usage: bundle_assert.sh <root>" >&2
  exit 2
fi

root=$1

expect_file() {
  local path=$1
  local expected=$2
  if [ ! -f "$path" ]; then
    echo "Missing file: $path" >&2
    exit 1
  fi
  local actual
  actual=$(cat "$path")
  if [ "$actual" != "$expected" ]; then
    echo "Unexpected content for $path" >&2
    exit 1
  fi
}

expect_file "$root/config-client/inbounds.json" '{"in":1}'
expect_file "$root/config-client/nested/route.json" '{"route":true}'
expect_file "$root/config-server/inbounds.json" '{"in":2}'
expect_file "$root/xp2p-client.toml" "client=1"
expect_file "$root/xp2p-server.toml" "server=1"
expect_file "$root/cert.pem" "cert"
expect_file "$root/key.pem" "key"
expect_file "$root/xp2p-client.state.json" '{"state":"c"}'
expect_file "$root/xp2p-server.state.json" '{"state":"s"}'
expect_file "$root/install-state-client.json" '{"install":"c"}'
expect_file "$root/install-state-server.json" '{"install":"s"}'
