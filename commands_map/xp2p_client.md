# xp2p client

## Global options (apply to all commands)
- --config, -c <path>             Path to configuration file
- --log-level, -l <level>         Override logging level
- --log-json, -j                  Emit logs in JSON format
- --version, -v                   Print xp2p version and exit (root only)

## Command tree

xp2p client
  Subcommands: install, remove, list, run, service, state, export, import, deploy, redirect, forward, reverse, mode, dns-forward (Linux only)

xp2p client install
Options:
Includes: Global options
- --path <dir>           Client installation directory (no short)
- --config-dir <dir>     Client configuration directory name (no short)
- --host <host>          Remote server host (no short)
- --port <port>          Remote server port (no short)
- --user <email>         Trojan user email (no short)
- --password <password>  Trojan password (no short)
- --sni <name>           TLS server name (SNI) (no short)
- --link <trojan://...>  Trojan client link (no short)
- --allow-insecure       Allow insecure TLS (skip verification) (no short)
- --strict-tls           Enforce TLS verification (no short)
- --force                Replace existing endpoint configuration (no short)

xp2p client remove [hostname|tag]
Options:
Includes: Global options
- --path <dir>        Client installation directory (no short)
- --config-dir <dir>  Client configuration directory name (no short)
- --keep-files        Keep installation files (only with --all) (no short)
- --ignore-missing    Do not fail if installation is absent (only with --all) (no short)
- --all               Remove all endpoints and configuration (no short)
- --quiet             Do not prompt for removal (no short)

xp2p client list
Options:
Includes: Global options
- --path <dir>       Client installation directory (no short)
- --config-dir <dir> Client configuration directory name (no short)

xp2p client run
Options:
Includes: Global options
- --path <dir>                Client installation directory (no short)
- --config-dir <dir>          Client configuration directory name (no short)
- --quiet                     Do not prompt for installation (no short)
- --auto-install              Install automatically if missing (no short)
- --xray-log-file <path>      File to append xray-core stderr output (no short)
- --heartbeat                 Enable background heartbeat probes (no short)
- --heartbeat-interval <dur>  Frequency of heartbeat probes (no short)
- --heartbeat-timeout <dur>   Timeout per heartbeat probe (no short)
- --heartbeat-port <port>     Diagnostics service port to probe (no short)
- --heartbeat-socks <host:port> SOCKS5 proxy for heartbeat (optional) (no short)

xp2p client service start
Options:
Includes: Global options
- (no local options)

xp2p client service stop
Options:
Includes: Global options
- (no local options)

xp2p client service status
Options:
Includes: Global options
- (no local options)

xp2p client service run
Options:
Includes: Global options
- --path <dir>                Client installation directory (no short)
- --config-dir <dir>          Client configuration directory name (no short)
- --log-file <path>           xp2p service log file (no short)
- --xray-log-file <path>      xray stderr log file (no short)
- --max-restarts <n>          Maximum restart attempts after failures (no short)
- --restart-delay <dur>       Delay between restart attempts (no short)
- --heartbeat                 Enable heartbeat probes (no short)
- --heartbeat-interval <dur>  Heartbeat interval (no short)
- --heartbeat-timeout <dur>   Heartbeat timeout (no short)
- --heartbeat-port <port>     Diagnostics service port to probe (no short)
- --heartbeat-socks <host:port> SOCKS5 proxy for heartbeat (optional) (no short)

xp2p client state
Options:
Includes: Global options
- --path <dir>       Client installation directory (no short)
- --watch            Continuously refresh state until interrupted (no short)
- --interval <dur>   Refresh interval for --watch (no short)
- --ttl <dur>        Heartbeat TTL for alive status (no short)

xp2p client export
Options:
Includes: Global options
- --config-root, -C <dir> Configuration root to export
- --output, -o <path>     Archive output path

xp2p client import
Options:
Includes: Global options
- --config-root, -C <dir> Configuration root to import into
- --input, -i <path> (required) Archive input path

xp2p client deploy
Options:
Includes: Global options
- --host <host> (required) Remote host (IP or DNS) to deploy (no short)
- --port <port>           Deploy port (default 62025) (no short)
- --install-dir <dir>     Server install directory override (no short)
- --user <email>          Trojan user identifier (email) (no short)
- --password <password>   Trojan user password (auto-generated when omitted) (no short)
- --trojan-port <port>    Trojan service port (no short)

xp2p client redirect add
Options:
Includes: Global options
- --path <dir>       Client installation directory (no short)
- --config-dir <dir> Client configuration directory name (no short)
- --cidr <cidr>      CIDR to redirect (mutually exclusive with --domain) (no short)
- --domain <name>    Domain to redirect (mutually exclusive with --cidr) (no short)
- --tag <tag>        Outbound tag to route through (prompts when omitted) (no short)
- --host <host>      Client endpoint hostname to route through (no short)
- --quiet            Do not prompt for outbound tags (no short)

xp2p client redirect remove
Options:
Includes: Global options
- --path <dir>       Client installation directory (no short)
- --config-dir <dir> Client configuration directory name (no short)
- --cidr <cidr>      CIDR mapping to remove (mutually exclusive with --domain) (no short)
- --domain <name>    Domain mapping to remove (mutually exclusive with --cidr) (no short)
- --tag <tag>        Outbound tag filter (prompts when omitted) (no short)
- --host <host>      Client endpoint hostname filter (no short)
- --quiet            Do not prompt for outbound tags (no short)

xp2p client redirect list
Options:
Includes: Global options
- --path <dir>       Client installation directory (no short)
- --config-dir <dir> Client configuration directory name (no short)

xp2p client forward add
Options:
Includes: Global options
- --path <dir>           Client installation directory (no short)
- --config-dir <dir>     Client configuration directory name (no short)
- --target <host:port> (required) Target host:port to forward traffic to (no short)
- --listen <host>        Local listen address (default 127.0.0.1) (no short)
- --listen-port <n>      Local listen port (auto-select when omitted) (no short)
- --proto <tcp|udp|both> Protocol to forward (no short)
- --base-port <n>        First port to probe when auto-selecting (no short)

xp2p client forward remove
Options:
Includes: Global options
- --path <dir>       Client installation directory (no short)
- --config-dir <dir> Client configuration directory name (no short)
- --listen-port <n>  Forward listen port (no short)
- --tag <tag>        Forward tag filter (no short)
- --remark <text>    Forward remark filter (no short)
- --ignore-missing   Do not fail when the forward rule does not exist (no short)
- --cleanup          Remove state entry even when config is missing (no short)

xp2p client forward list
Options:
Includes: Global options
- --path <dir>       Client installation directory (no short)
- --config-dir <dir> Client configuration directory name (no short)

xp2p client reverse
Options:
Includes: Global options
- --path <dir>       Client installation directory (no short)
- --config-dir <dir> Client configuration directory name (no short)

xp2p client reverse list
Options:
Includes: Global options
- --path <dir>       Client installation directory (no short)
- --config-dir <dir> Client configuration directory name (no short)

xp2p client mode [tun|proxy]
Options:
Includes: Global options
- --path <dir>       Client installation directory (no short)
- --config-dir <dir> Client configuration directory name (no short)
- --config <path>    Path to configuration file (toml) (no short)

xp2p client dns-forward (Linux only)
  Subcommands: add, remove, list

xp2p client dns-forward add
Options:
Includes: Global options
- --domain <name> (required) Domain name to match (no short)
- --target <ip:port> (required) Upstream DNS server (no short)
- --with-forward      Create or reuse a port forward for the target (no short)
- --intercept         Install DNS intercept redirect (53/tcp,udp) (no short)
- --quiet             Suppress interactive prompts (no short)
- --debug             Emit diagnostics output on error (no short)

xp2p client dns-forward remove
Options:
Includes: Global options
- --domain <name>     Domain name to remove (no short)
- --with-forward      Remove an auto-created port forward (no short)
- --intercept         Remove DNS intercept redirect (no short)
- --all               Remove all managed DNS forward entries (no short)
- --quiet             Suppress interactive prompts (no short)
- --debug             Emit diagnostics output on error (no short)

xp2p client dns-forward list
Options:
Includes: Global options
- --debug             Emit diagnostics output on error (no short)
