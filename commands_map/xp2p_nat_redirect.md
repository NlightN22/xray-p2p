# xp2p nat-redirect

## Command tree

xp2p nat-redirect
  Summary: Manage transparent NAT redirect rules (Linux only)
  Subcommands: add, remove, list
Options:
Includes: inherited options

xp2p nat-redirect add
  Summary: Add transparent redirect rules for a CIDR
Options:
Includes: inherited options
- --cidr, -C <cidr> destination CIDR
- --entry-dir, -E <string> entry directory for nftables snippet generation
- --inbounds, -i <string> path to inbounds.json used for auto port detection
- --port, -P <port> dokodemo-door port to redirect to (auto-detected when omitted)
- --print-only, -O render firewall changes without applying them
- --quiet, -q avoid interactive prompts when auto-selecting dokodemo port
- --snippet, -s <string> nftables snippet path

xp2p nat-redirect remove
  Summary: Remove transparent redirect rules
Options:
Includes: inherited options
- --all, -a remove all transparent redirects
- --cidr, -C <cidr> destination CIDR
- --entry-dir, -E <string> entry directory for nftables snippet generation
- --print-only, -O render firewall changes without applying them
- --snippet, -s <string> nftables snippet path

xp2p nat-redirect list
  Summary: List transparent redirect entries
Options:
Includes: inherited options

