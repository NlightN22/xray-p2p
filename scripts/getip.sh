#!/bin/sh
# Try multiple sources (DNS + HTTP) and pick consensus
# Fallback to first working if no consensus.

# DNS sources (will be empty if dig/nslookup missing)
ip_opendns=$(dig +short myip.opendns.com @resolver1.opendns.com 2>/dev/null || true)
ip_cf=$(dig +short whoami.cloudflare @1.1.1.1 2>/dev/null || true)

# HTTP fallbacks
ip_ifconfig=$(curl -fsS https://ifconfig.me || echo "")
ip_checkip=$(curl -fsS https://checkip.amazonaws.com || echo "")

# Collect valid IPv4 addresses
all=$(printf "%s\n%s\n%s\n%s\n" "$ip_opendns" "$ip_cf" "$ip_ifconfig" "$ip_checkip" \
  | grep -E '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' || true)

# Consensus: most common
ext_ip=$(printf "%s\n" "$all" | sort | uniq -c | sort -rn | head -n1 | awk '{print $2}')

# Fallback to first non-empty if consensus failed
[ -z "$ext_ip" ] && ext_ip=$(printf "%s\n%s\n%s\n%s\n" "$ip_opendns" "$ip_cf" "$ip_ifconfig" "$ip_checkip" \
  | grep -E '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' | head -n1)

printf "%s\n" "$ext_ip"
