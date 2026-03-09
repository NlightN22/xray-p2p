# xp2p server

## Global options (apply to all commands)
- --config, -c <path>             Path to configuration file
- --log-level, -l <level>         Override logging level
- --log-json, -j                  Emit logs in JSON format
- --version, -v                   Print xp2p version and exit (root only)
- --diag-service-port, -P <port>  Diagnostics service port
- --diag-service-mode, -M <auto|manual>  Diagnostics service startup mode

## Command tree

xp2p server
  Subcommands: install, remove, run, service, state, export, import, user, redirect, forward, reverse, cert, deploy, mode, dns-forward (Linux only)

xp2p server install
Options:
Includes: Global options
- --path <dir>       Server installation directory (no short)
- --config-dir <dir> Server configuration directory name (no short)
- --port <port>      Server listener port (no short)
- --cert-store <ref> TLS certificate store reference (win-store) (no short)
- --cert <path>      TLS certificate file to deploy (no short)
- --key <path>       TLS private key file to deploy (no short)
- --host <host>      Public host name or IP for generated configuration (no short)
- --force            Overwrite existing installation (no short)

xp2p server remove
Options:
Includes: Global options
- --path <dir>       Server installation directory (no short)
- --config-dir <dir> Server configuration directory name (no short)
- --keep-files       Keep installation files (no short)
- --ignore-missing   Do not fail if service or files are absent (no short)
- --quiet            Do not prompt for removal (no short)

xp2p server run
Options:
Includes: Global options
- --path <dir>          Server installation directory (no short)
- --config-dir <dir>    Server configuration directory name (no short)
- --auto-install        Install server assets when missing without prompting (no short)
- --quiet               Suppress interactive prompts (no short)
- --xray-log-file <path> Append xray stderr output to file (no short)

xp2p server service start
Options:
Includes: Global options
- (no local options)

xp2p server service stop
Options:
Includes: Global options
- (no local options)

xp2p server service status
Options:
Includes: Global options
- (no local options)

xp2p server service run
Options:
Includes: Global options
- --path <dir>       Server installation directory (no short)
- --config-dir <dir> Server configuration directory name (no short)
- --log-file <path>  xp2p service log file (no short)
- --xray-log-file <path> xray stderr log file (no short)
- --max-restarts <n> Maximum restart attempts after failures (no short)
- --restart-delay <dur> Delay between restart attempts (no short)

xp2p server state
Options:
Includes: Global options
- --path <dir>       Server installation directory (no short)
- --watch            Continuously refresh state until interrupted (no short)
- --interval <dur>   Refresh interval for --watch (no short)
- --ttl <dur>        Heartbeat TTL for alive status (no short)

xp2p server export
Options:
Includes: Global options
- --config-root, -C <dir> Configuration root to export
- --output, -o <path>     Archive output path

xp2p server import
Options:
Includes: Global options
- --config-root, -C <dir> Configuration root to import into
- --input, -i <path> (required) Archive input path

xp2p server user add
Options:
Includes: Global options
- --path <dir>       Server installation directory (no short)
- --config-dir <dir> Server configuration directory name or absolute path (no short)
- --id <id>          Trojan client identifier (derives the <id><host>.rev reverse tag) (no short)
- --password <password> Trojan client password or pre-shared key (auto-generated when omitted) (no short)
- --key <password>   Alias for --password (no short)
- --host <host>      Public host name or IP for generated connection link (no short)
- --no-reverse       Skip creating reverse portal/routing entries (no short)
- --force            Overwrite existing user entry (no short)

xp2p server user remove
Options:
Includes: Global options
- --path <dir>       Server installation directory (no short)
- --config-dir <dir> Server configuration directory name or absolute path (no short)
- --id <id> (required) Trojan client identifier (no short)
- --host <host>      Public host name or IP (defaults to server host) (no short)

xp2p server user list
Options:
Includes: Global options
- --path <dir>       Server installation directory (no short)
- --config-dir <dir> Server configuration directory name or absolute path (no short)
- --host <host>      Public host name or IP for generated connection links (no short)

xp2p server redirect add
Options:
Includes: Global options
- --path <dir>       Server installation directory (no short)
- --config-dir <dir> Server configuration directory name or absolute path (no short)
- --cidr <cidr>      CIDR to redirect (mutually exclusive with --domain) (no short)
- --domain <name>    Domain to redirect (mutually exclusive with --cidr) (no short)
- --tag <tag>        Reverse outbound tag to route through (prompts when omitted) (no short)
- --host <host>      Reverse portal host to route through (no short)
- --quiet            Do not prompt for outbound tags (no short)

xp2p server redirect remove
Options:
Includes: Global options
- --path <dir>       Server installation directory (no short)
- --config-dir <dir> Server configuration directory name or absolute path (no short)
- --cidr <cidr>      CIDR mapping to remove (mutually exclusive with --domain) (no short)
- --domain <name>    Domain mapping to remove (mutually exclusive with --cidr) (no short)
- --tag <tag>        Reverse outbound tag filter (prompts when omitted) (no short)
- --host <host>      Reverse portal host filter (no short)
- --quiet            Do not prompt for outbound tags (no short)

xp2p server redirect list
Options:
Includes: Global options
- --path <dir>       Server installation directory (no short)
- --config-dir <dir> Server configuration directory name or absolute path (no short)

xp2p server forward add
Options:
Includes: Global options
- --path <dir>           Server installation directory (no short)
- --config-dir <dir>     Server configuration directory name or absolute path (no short)
- --target <host:port> (required) Target host:port to forward traffic to (no short)
- --listen <host>        Local listen address (default 127.0.0.1) (no short)
- --listen-port <n>      Local listen port (auto-select when omitted) (no short)
- --proto <tcp|udp|both> Protocol to forward (no short)
- --base-port <n>        First port to probe when auto-selecting (no short)

xp2p server forward remove
Options:
Includes: Global options
- --path <dir>       Server installation directory (no short)
- --config-dir <dir> Server configuration directory name or absolute path (no short)
- --listen-port <n>  Forward listen port (no short)
- --tag <tag>        Forward tag filter (no short)
- --remark <text>    Forward remark filter (no short)
- --ignore-missing   Do not fail when the forward rule does not exist (no short)

xp2p server forward list
Options:
Includes: Global options
- --path <dir>       Server installation directory (no short)
- --config-dir <dir> Server configuration directory name or absolute path (no short)

xp2p server reverse
Options:
Includes: Global options
- --path <dir>       Server installation directory (no short)
- --config-dir <dir> Server configuration directory name or absolute path (no short)

xp2p server reverse list
Options:
Includes: Global options
- --path <dir>       Server installation directory (no short)
- --config-dir <dir> Server configuration directory name or absolute path (no short)

xp2p server cert set
Options:
Includes: Global options
- --path <dir>       Server installation directory (no short)
- --config-dir <dir> Server configuration directory name or absolute path (no short)
- --cert-store <ref> TLS certificate store reference (win-store) (no short)
- --cert <path>      TLS certificate file to deploy (no short)
- --key <path>       TLS private key file to deploy (no short)
- --host <host>      Public host name or IP for certificate generation (no short)
- --force            Overwrite existing TLS configuration without prompting (no short)

xp2p server cert state
Options:
Includes: Global options
- --path <dir>       Server installation directory (no short)
- --config-dir <dir> Server configuration directory name or absolute path (no short)

xp2p server deploy
Options:
Includes: Global options
- --listen <host:port> Deploy listen address (default :62025) (no short)
- --link <trojan://...> (required) Deploy link (no short)
- --timeout <dur> Idle shutdown timeout (no short)

xp2p server mode [tun|proxy]
Options:
Includes: Global options
- --path <dir>       Server installation directory (no short)
- --config-dir <dir> Server configuration directory name (no short)
- --config <path>    Path to configuration file (toml) (no short)

xp2p server dns-forward (Linux only)
  Subcommands: add, remove, list

xp2p server dns-forward add
Options:
Includes: Global options
- --domain <name> (required) Domain name to match (no short)
- --target <ip:port> (required) Upstream DNS server (no short)
- --with-forward      Create or reuse a port forward for the target (no short)
- --intercept         Install DNS intercept redirect (53/tcp,udp) (no short)
- --quiet             Suppress interactive prompts (no short)
- --debug             Emit diagnostics output on error (no short)

xp2p server dns-forward remove
Options:
Includes: Global options
- --domain <name>     Domain name to remove (no short)
- --with-forward      Remove an auto-created port forward (no short)
- --intercept         Remove DNS intercept redirect (no short)
- --all               Remove all managed DNS forward entries (no short)
- --quiet             Suppress interactive prompts (no short)
- --debug             Emit diagnostics output on error (no short)

xp2p server dns-forward list
Options:
Includes: Global options
- --debug             Emit diagnostics output on error (no short)
