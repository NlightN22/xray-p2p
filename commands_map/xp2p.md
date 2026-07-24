# xp2p

## Global options (apply to all commands)
- --help, -h show help for command
- --config, -c <path> path to configuration file
- --json, -J emit command result as JSON
- --log-json, -j emit logs in JSON format
- --log-level, -l <string> override logging level
- --version, -v print xp2p version and exit

## Command tree

xp2p
  Summary: Cross-platform helper for XRAY-P2P
  Subcommands: client, server, diag, ping, heartbeat, completion, docs, nat-redirect

xp2p client
  Summary: Manage xp2p client installation
  Subcommands: install, disable, enable, update, remove, list, run, service, state, obs, render, debug, export, import, deploy, redirect, forward, reverse, mode, group, subscription, dns-forward
Options:
Includes: inherited options

xp2p server
  Summary: Manage xp2p server components
  Subcommands: install, remove, run, service, state, render, debug, export, import, user, identity, redirect, forward, reverse, cert, deploy, mode, profile, ha, dns-forward
Options:
Includes: inherited options

xp2p diag
  Summary: Run diagnostics responder in the foreground
  Machine output: lifecycle
  Machine output note: the command runs a foreground diagnostics service
Options:
Includes: inherited options
- --listen, -n <host:port> listen address (host:port)
- --quiet, -q reduce log output

xp2p ping <host>
  Summary: Send diagnostic ping requests to xp2p agents
  Machine output: streaming
  Machine output note: continuous ping output requires a future NDJSON contract
Options:
Includes: inherited options
- --continuous, -C send ping requests until interrupted
- --count, -N <n> number of echo requests to send
- --endpoint, -e <id> endpoint tag to use when multiple endpoints share the same host
- --index, -i <n> endpoint index (1-based) to use when multiple endpoints share the same host
- --port, -P <port> target port (default 62022)
- --proto, -o <proto> transport protocol for compatibility (tcp or https)
- --timeout, -t <duration> per-request timeout in seconds (optional)
- --tunnel, -T <string> route ping through xp2p tunnel (SOCKS5 host:port); omit value to auto-detect from xp2p config

xp2p heartbeat
  Summary: Inspect the heartbeat protocol contract
  Subcommands: contract
Options:
Includes: inherited options

xp2p completion [bash|zsh|fish|powershell]
  Summary: Generate shell completion scripts
  Machine output: generator
  Machine output note: the result is a shell completion script
Options:
Includes: inherited options

xp2p docs
  Summary: Generate CLI reference documentation
  Subcommands: command-map
Options:
Includes: inherited options
- --dir, -d <dir> (required) destination directory for generated docs

xp2p nat-redirect
  Summary: Manage transparent NAT redirect rules (Linux only)
  Subcommands: add, remove, list
Options:
Includes: inherited options
