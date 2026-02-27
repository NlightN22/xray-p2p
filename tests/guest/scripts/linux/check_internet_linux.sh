#!/bin/sh
set -eu

dns_name="${1:-example.com}"
tcp_host="${2:-1.1.1.1}"
tcp_port="${3:-443}"

if command -v nslookup >/dev/null 2>&1; then
  if ! nslookup "$dns_name" >/dev/null 2>&1; then
    echo "Internet check failed: DNS lookup for $dns_name" >&2
    exit 1
  fi
elif command -v getent >/dev/null 2>&1; then
  if ! getent hosts "$dns_name" >/dev/null 2>&1; then
    echo "Internet check failed: DNS lookup for $dns_name" >&2
    exit 1
  fi
else
  echo "Internet check failed: DNS tool missing" >&2
  exit 1
fi

if command -v nc >/dev/null 2>&1; then
  if nc -z -w 3 "$tcp_host" "$tcp_port" >/dev/null 2>&1; then
    exit 0
  fi
elif command -v timeout >/dev/null 2>&1 && command -v bash >/dev/null 2>&1; then
  if timeout 3 bash -lc "exec 3<>/dev/tcp/$tcp_host/$tcp_port" >/dev/null 2>&1; then
    exit 0
  fi
elif command -v wget >/dev/null 2>&1; then
  if wget -q -O /dev/null "http://$dns_name" >/dev/null 2>&1; then
    exit 0
  fi
elif command -v curl >/dev/null 2>&1; then
  if curl -s --max-time 3 "http://$dns_name" >/dev/null 2>&1; then
    exit 0
  fi
fi

echo "Internet check failed: TCP connect to $tcp_host:$tcp_port" >&2
exit 1
