#!/bin/bash
set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "Usage: bundle_setup.sh <root>" >&2
  exit 2
fi

root=$1

mkdir -p "$root/config-client" "$root/config-client/nested" "$root/config-server"

cat >"$root/config-client/inbounds.json" <<'EOF'
{"in":1}
EOF

cat >"$root/config-client/nested/route.json" <<'EOF'
{"route":true}
EOF

cat >"$root/config-server/inbounds.json" <<'EOF'
{"in":2}
EOF

printf '%s\n' 'client=1' >"$root/xp2p-client.toml"
printf '%s\n' 'server=1' >"$root/xp2p-server.toml"
printf '%s\n' 'cert' >"$root/cert.pem"
printf '%s\n' 'key' >"$root/key.pem"
printf '%s\n' '{"state":"c"}' >"$root/xp2p-client.state.json"
printf '%s\n' '{"state":"s"}' >"$root/xp2p-server.state.json"
printf '%s\n' '{"install":"c"}' >"$root/install-state-client.json"
printf '%s\n' '{"install":"s"}' >"$root/install-state-server.json"
