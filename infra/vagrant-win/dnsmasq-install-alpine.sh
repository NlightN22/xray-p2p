#!/bin/sh
# Alpine dnsmasq installer & config writer (POSIX sh)
# Notes: no "pipefail", compatible with BusyBox ash
# Command to install at PS
# Get-Content .\dnsmasq-install-alpine.sh -Raw |  % { $_ -replace "`r","" } |  vagrant ssh c1 -c "sh -s"

set -eu  # -x optional for debug

# --- Adjust these if needed ---
ZONE="corp.test.com"            # local authoritative zone
ETH_IFACE="${ETH_IFACE:-eth1}"  # Interface to detect IPv4 from
LOGFILE="/var/log/dnsmasq.log"
# ------------------------------

# 0) Require HOST_FQDN as first argument
if [ "$#" -lt 1 ] || [ -z "$1" ]; then
  echo "Usage: $0 HOST_FQDN" >&2
  exit 1
fi
HOST_FQDN=$1

# 1) Determine privilege escalation helper
if [ "$(id -u)" -eq 0 ]; then
  AS_ROOT=""
elif command -v sudo >/dev/null 2>&1; then
  AS_ROOT="sudo"
elif command -v doas >/dev/null 2>&1; then
  AS_ROOT="doas"
else
  echo "This script requires root privileges. Install sudo/doas or run as root." >&2
  exit 1
fi

run_root() {
  if [ -z "$AS_ROOT" ]; then
    "$@"
  else
    "$AS_ROOT" "$@"
  fi
}

# 2) Verify Alpine
if ! grep -qi "alpine" /etc/os-release 2>/dev/null; then
  echo "This script is intended for Alpine Linux." >&2
  exit 1
fi

# 3) Detect IPv4 on the target interface (used for A record + listen address)
A_IP=${A_IP:-}
LISTEN_IPS=${LISTEN_IPS:-}
IFACE_IP=$(ip -4 -o addr show dev "$ETH_IFACE" scope global | awk '{print $4}' | cut -d/ -f1 | head -n1)

if [ -z "$IFACE_IP" ]; then
  echo "Failed to detect IPv4 address on interface $ETH_IFACE." >&2
  echo "Set A_IP manually or ensure the interface has an address." >&2
  exit 1
fi

if [ -z "$A_IP" ]; then
  A_IP="$IFACE_IP"
fi

if [ -z "$LISTEN_IPS" ]; then
  LISTEN_IPS="127.0.0.1,$IFACE_IP"
fi

# 4) Install dnsmasq
run_root apk update
run_root apk add --no-cache dnsmasq

# 5) Backup existing config if present
if run_root test -f /etc/dnsmasq.conf; then
  run_root cp -a /etc/dnsmasq.conf "/etc/dnsmasq.conf.bak.$(date +%Y%m%d-%H%M%S)"
fi

# 6) Write new config (authoritative zone; AAAA -> NOERROR/NODATA)
cat <<EOF | run_root tee /etc/dnsmasq.conf >/dev/null
# /etc/dnsmasq.conf (managed by script)
no-resolv
no-poll

# Bind to specific addresses (adjust if needed)
listen-address=${LISTEN_IPS}

# Authoritative local zone to avoid NXDOMAIN on AAAA
auth-zone=${ZONE}
auth-server=${HOST_FQDN},${A_IP}

# Define host records (A only; AAAA will be NOERROR/NODATA)
host-record=${HOST_FQDN},${A_IP}
host-record=r1.corp.test.com,10.0.101.1

# Optional logging
log-queries
log-facility=${LOGFILE}
EOF

# 7) Prepare log file
run_root mkdir -p "$(dirname "$LOGFILE")"
run_root install -m 0644 /dev/null "$LOGFILE"

# 8) Enable and restart service
run_root rc-update add dnsmasq default >/dev/null 2>&1 || true
run_root rc-service dnsmasq restart

echo "dnsmasq configured:"
echo "  zone      : ${ZONE}"
echo "  host      : ${HOST_FQDN} -> ${A_IP}"
echo "  listen    : ${LISTEN_IPS}"
echo "  log       : ${LOGFILE}"
