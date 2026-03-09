# xp2p nat-redirect

xp2p nat-redirect (Linux only)
  Subcommands: add, remove, list

xp2p nat-redirect add
Options:
- --cidr <cidr> (required) Destination CIDR (no short)
- --port <n>              Dokodemo-door port to redirect to (auto-detected when omitted) (no short)
- --print-only            Render firewall changes without applying them (no short)
- --quiet                 Avoid interactive prompts when auto-selecting dokodemo port (no short)
- --snippet <path>        Nftables snippet path (no short)
- --entry-dir <path>      Entry directory for nftables snippet generation (no short)
- --inbounds <path>       Path to inbounds.json used for auto port detection (no short)
- Global options

xp2p nat-redirect remove
Options:
- --cidr <cidr>    Destination CIDR (no short)
- --all            Remove all transparent redirects (no short)
- --print-only     Render firewall changes without applying them (no short)
- --snippet <path> Nftables snippet path (no short)
- --entry-dir <path> Entry directory for nftables snippet generation (no short)
- Global options

xp2p nat-redirect list
Options:
- Global options
