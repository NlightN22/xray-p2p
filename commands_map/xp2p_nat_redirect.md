# xp2p nat-redirect

xp2p nat-redirect (Linux only)
  Subcommands: add, remove, list

xp2p nat-redirect add
Options:
- --cidr, -C <cidr> (required) Destination CIDR
- --port, -P <n>              Dokodemo-door port to redirect to (auto-detected when omitted)
- --print-only, -O            Render firewall changes without applying them
- --quiet, -q                 Avoid interactive prompts when auto-selecting dokodemo port
- --snippet, -s <path>        Nftables snippet path
- --entry-dir, -E <path>      Entry directory for nftables snippet generation
- --inbounds, -i <path>       Path to inbounds.json used for auto port detection
- Global options

xp2p nat-redirect remove
Options:
- --cidr, -C <cidr>    Destination CIDR
- --all, -a            Remove all transparent redirects
- --print-only, -O     Render firewall changes without applying them
- --snippet, -s <path> Nftables snippet path
- --entry-dir, -E <path> Entry directory for nftables snippet generation
- Global options

xp2p nat-redirect list
Options:
- Global options
