# xp2p

## Global options (apply to all commands)
- --config, -c <path>             Path to configuration file
- --log-level, -l <level>         Override logging level
- --log-json, -j                  Emit logs in JSON format
- --version, -v                   Print xp2p version and exit (root only)
- --diag-service-port, -P <port>  Diagnostics service port
- --diag-service-mode, -M <auto|manual>  Diagnostics service startup mode

## Client override options (apply to all `xp2p client ...` subcommands)
- --client-install-dir, -I <dir>   Client installation directory (Windows)
- --client-config-dir, -D <dir>    Client configuration directory name
- --client-host, -A <host>         Remote server host for client config
- --client-port, -R <port>         Remote server port for client config
- --client-user, -U <email>        Trojan user email for client config
- --client-password, -W <password> Trojan password for client config
- --client-sni, -N <name>          TLS server name (SNI) for client config
- --client-allow-insecure, -K      Allow TLS verification to be skipped
- --client-strict-tls, -T          Enforce TLS verification

## Server override options (apply to all `xp2p server ...` subcommands)
- --server-install-dir, -I <dir>   Server installation directory (Windows)
- --server-config-dir, -D <dir>    Server configuration directory name
- --server-cert-store, -S <ref>    TLS certificate store reference (win-store)
- --server-cert, -E <path>         Path to TLS certificate file (PEM)
- --server-key, -K <path>          Path to TLS private key file (PEM)
- --server-host, -H <host>         Public host name or IP for server certificate and links

## Command tree

xp2p
  Behavior: Starts diagnostics service in background and waits (no args).
  Options: Global options

xp2p completion [bash|zsh|fish|powershell]
  Options: Global options

xp2p docs
  Options:
  - --dir <path> (required) Destination directory for generated docs (no short)
  - Global options

xp2p diag
  Options:
  - --listen <host:port> Listen address (host:port) (no short)
  - --proto <tcp|udp>    Protocol to listen on (no short)
  - --quiet              Reduce log output (no short)
  - Global options

xp2p ping <host>
  Options:
  - --count <n>          Number of echo requests to send (no short)
  - --timeout <sec>      Per-request timeout in seconds (no short)
  - --proto <tcp|udp>    Protocol to use (no short)
  - --port <n>           Target port (default 62022) (no short)
  - --tunnel [host:port] Route ping through xp2p tunnel (SOCKS5). If value omitted, auto-detect from config (no short)
  - --endpoint <tag>     Endpoint tag to use when multiple endpoints share the same host (no short)
  - --index <n>          Endpoint index (1-based) to use when multiple endpoints share the same host (no short)
  - Global options

xp2p nat-redirect (Linux only)
  Subcommands: add, remove, list

xp2p client
  Subcommands: install, remove, list, run, service, state, export, import, deploy, redirect, forward, reverse, mode, dns-forward (Linux only)

xp2p server
  Subcommands: install, remove, run, service, state, export, import, user, redirect, forward, reverse, cert, deploy, mode, dns-forward (Linux only)
