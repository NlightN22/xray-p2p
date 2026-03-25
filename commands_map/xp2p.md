# xp2p

## Global options (apply to all commands)
- --help, -h                     Show help for command
- --config, -c <path>             Path to configuration file
- --log-level, -l <level>         Override logging level (debug|info|warn|error)
- --log-json, -j                  Emit logs in JSON format
- --version, -v                   Print xp2p version and exit (root only)

## Command tree

xp2p
  Behavior: Show help and exit.
  Options: Global options

xp2p completion [bash|zsh|fish|powershell]
  Options: Global options

xp2p docs
  Options:
  - --dir, -d <path> (required) Destination directory for generated docs
  - Global options

xp2p diag
  Options:
  - --listen, -n <host:port> Listen address (host:port)
  - --proto, -o <tcp|udp>    Protocol to listen on
  - --quiet, -q              Reduce log output
  - Global options

xp2p ping <host>
  Options:
  - --count, -N <n>          Number of echo requests to send
  - --timeout, -t <sec>      Per-request timeout in seconds
  - --proto, -o <tcp|udp>    Protocol to use
  - --port, -P <n>           Target port (default 62022)
  - --tunnel, -T [host:port] Route ping through xp2p tunnel (SOCKS5). If value omitted, auto-detect from config
  - --endpoint, -e <tag>     Endpoint tag to use when multiple endpoints share the same host
  - --index, -i <n>          Endpoint index (1-based) to use when multiple endpoints share the same host
  - Global options

xp2p nat-redirect (Linux only)
  Subcommands: add, remove, list

xp2p client
  Subcommands: install, remove, list, run, service, state, export, import, deploy, redirect, forward, reverse, mode, dns-forward (Linux only)

xp2p server
  Subcommands: install, remove, run, service, state, export, import, user, redirect, forward, reverse, cert, deploy, mode, dns-forward (Linux only)
