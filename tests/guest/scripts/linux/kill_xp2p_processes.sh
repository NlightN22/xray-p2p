#!/bin/sh
set -eu

for pattern in \
  "xp2p client run" \
  "xp2p server run" \
  "xp2p client deploy" \
  "xp2p server deploy" \
  "/etc/xp2p/bin/xray" \
  "/usr/bin/xp2p"
do
  pkill -f "$pattern" >/dev/null 2>&1 || true
done
