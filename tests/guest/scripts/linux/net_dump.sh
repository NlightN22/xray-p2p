#!/bin/sh
set -eu

echo "== ip addr =="
ip -o -4 addr show || true
echo "== ip route =="
ip route show || true
