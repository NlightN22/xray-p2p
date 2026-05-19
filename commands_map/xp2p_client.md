# xp2p client

## Global options (apply to all commands)
- --help, -h                     Show help for command
- --config, -c <path>             Path to configuration file
- --log-level, -l <level>         Override logging level (debug|info|warn|error)
- --log-json, -j                  Emit logs in JSON format
- --version, -v                   Print xp2p version and exit (root only)

## Command tree

xp2p client
  Subcommands: install, remove, list, run, service, state, render, debug, export, import, deploy, redirect, forward, reverse, mode, dns-forward (Linux only)

xp2p client install
Options:
Includes: Global options
- --path, -p <dir>           Client installation directory
- --config-dir, -D <dir>     Client configuration directory name
- --host, -H <host>          Remote server host
- --port, -P <port>          Remote server port
- --user, -u <email>         Trojan user email
- --password, -w <password>  Trojan password
- --sni, -s <name>           TLS server name (SNI)
- --link, -L <trojan://...>  Trojan client link
- --allow-insecure, -I       Allow insecure TLS (skip verification)
- --strict-tls, -S           Enforce TLS verification
- --force, -f                Replace existing endpoint configuration
- --mode, -M <proxy|tun|tun:split|tun:full> Target client mode (optional)
- --tun-mode, -m <split|full> TUN routing mode (default: split)

xp2p client remove [hostname|tag]
Options:
Includes: Global options
- --path, -p <dir>        Client installation directory
- --config-dir, -D <dir>  Client configuration directory name
- --keep-files, -K        Keep installation files (only with --all)
- --ignore-missing, -m    Do not fail if installation is absent (only with --all)
- --all, -a               Remove all endpoints and configuration
- --quiet, -q             Do not prompt for removal

xp2p client list
Options:
Includes: Global options
- --path, -p <dir>       Client installation directory
- --config-dir, -D <dir> Client configuration directory name
- --pending, -y          List pending configuration

xp2p client run
Options:
Includes: Global options
- --path, -p <dir>                Client installation directory
- --config-dir, -D <dir>          Client configuration directory name
- --quiet, -q                     Do not prompt for installation
- --auto-install, -A              Install automatically if missing
- --verbose, -V                   Emit full-tunnel change details
- --heartbeat, -b                 Enable background heartbeat probes
- --heartbeat-interval, -I <dur>  Frequency of heartbeat probes
- --heartbeat-timeout, -T <dur>   Timeout per heartbeat probe
- --heartbeat-port, -P <port>     Diagnostics service port to probe
- --heartbeat-socks, -S <host:port> SOCKS5 proxy for heartbeat (optional)

xp2p client service start
Options:
Includes: Global options
- On Windows, --log-level updates the service environment (XP2P_LOG_LEVEL)

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
- --path, -p <dir>                Client installation directory
- --config-dir, -D <dir>          Client configuration directory name
- --log-file, -F <path>           xp2p service log file
- --max-restarts, -R <n>          Maximum restart attempts after failures
- --restart-delay, -r <dur>       Delay between restart attempts
- --heartbeat, -b                 Enable heartbeat probes
- --verbose, -V                   Emit full-tunnel change details
- --heartbeat-interval, -I <dur>  Heartbeat interval
- --heartbeat-timeout, -T <dur>   Heartbeat timeout
- --heartbeat-port, -P <port>     Diagnostics service port to probe
- --heartbeat-socks, -S <host:port> SOCKS5 proxy for heartbeat (optional)

xp2p client state
Options:
Includes: Global options
- --path, -p <dir>       Client installation directory
- --pending, -y          Show pending configuration
- --watch, -w            Continuously refresh state until interrupted
- --interval, -i <dur>   Refresh interval for --watch
- --ttl, -T <dur>        Heartbeat TTL for alive status

xp2p client render
  Subcommands: xray

xp2p client render xray
Options:
Includes: Global options
- --live, -L             Render live runtime artifacts
- --desired, -d          Compile Desired inputs without applying
- --output, -o <path>    Output path ('-' for stdout)

xp2p client debug
  Subcommands: bundle

xp2p client debug bundle
Options:
Includes: Global options
- --output, -o <path>    Archive output path

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
- --host, -H <host> (required) Remote host (IP or DNS) to deploy
- --port, -P <port>           Deploy port (default 62025)
- --install-dir, -I <dir>     Server install directory override
- --user, -u <email>          Trojan user identifier (email)
- --password, -w <password>   Trojan user password (auto-generated when omitted)
- --trojan-port, -T <port>    Trojan service port
- --mode, -M <proxy|tun|tun:split|tun:full> Target client mode (optional)
- --tun-mode, -m <split|full> TUN routing mode (split or full)
- --force, -f                 Allow changing existing tun mode

xp2p client redirect add
Options:
Includes: Global options
- --path, -p <dir>       Client installation directory
- --config-dir, -D <dir> Client configuration directory name
- --cidr, -C <cidr>      CIDR to redirect (mutually exclusive with --domain)
- --domain, -d <name>    Domain to redirect (mutually exclusive with --cidr)
- --tag, -g <tag>        Outbound tag to route through (prompts when omitted)
- --host, -H <host>      Client endpoint hostname to route through
- --no-routes, -N        Do not add OS routes for CIDR redirects
- --quiet, -q            Do not prompt for outbound tags

xp2p client redirect remove
Options:
Includes: Global options
- --path, -p <dir>       Client installation directory
- --config-dir, -D <dir> Client configuration directory name
- --cidr, -C <cidr>      CIDR mapping to remove (mutually exclusive with --domain)
- --domain, -d <name>    Domain mapping to remove (mutually exclusive with --cidr)
- --tag, -g <tag>        Outbound tag filter (prompts when omitted)
- --host, -H <host>      Client endpoint hostname filter
- --quiet, -q            Do not prompt for outbound tags

xp2p client redirect list
Options:
Includes: Global options
- --path, -p <dir>       Client installation directory
- --config-dir, -D <dir> Client configuration directory name
- --pending, -y          List pending configuration

xp2p client forward add
Options:
Includes: Global options
- --path, -p <dir>           Client installation directory
- --config-dir, -D <dir>     Client configuration directory name
- --target, -t <host:port> (required) Target host:port to forward traffic to
- --listen, -n <host>        Local listen address (default 127.0.0.1)
- --listen-port, -P <n>      Local listen port (auto-select when omitted)
- --proto, -o <tcp|udp|both> Protocol to forward
- --base-port, -B <n>        First port to probe when auto-selecting

xp2p client forward remove
Options:
Includes: Global options
- --path, -p <dir>       Client installation directory
- --config-dir, -D <dir> Client configuration directory name
- --listen-port, -P <n>  Forward listen port
- --tag, -g <tag>        Forward tag filter
- --remark, -r <text>    Forward remark filter
- --ignore-missing, -m   Do not fail when the forward rule does not exist
- --cleanup, -C          Remove state entry even when config is missing

xp2p client forward list
Options:
Includes: Global options
- --path, -p <dir>       Client installation directory
- --config-dir, -D <dir> Client configuration directory name
- --pending, -y          List pending configuration

xp2p client reverse
Options:
Includes: Global options
- --path, -p <dir>       Client installation directory
- --config-dir, -D <dir> Client configuration directory name
- --pending, -y          List pending configuration

xp2p client reverse list
Options:
Includes: Global options
- --path, -p <dir>       Client installation directory
- --config-dir, -D <dir> Client configuration directory name
- --pending, -y          List pending configuration

xp2p client mode [tun|proxy] [split|full]
Options:
Includes: Global options
- --path, -p <dir>       Client installation directory
- --config-dir, -D <dir> Client configuration directory name
- --tag, -g <tag>        Outbound tag for full-tunnel routing (prompts when omitted)
- --host, -H <host>      Client endpoint hostname for full-tunnel routing
- --quiet, -q            Do not prompt for outbound tags
- --verbose, -V          Emit full-tunnel change details

xp2p client dns-forward (Linux only)
  Subcommands: add, remove, list

xp2p client dns-forward add
Options:
Includes: Global options
- --domain, -d <name> (required) Domain name to match
- --target, -t <ip:port> (required) Upstream DNS server
- --with-forward, -W      Deprecated; dns-forward always ensures a target forward
- --intercept, -I         Install DNS intercept redirect (53/tcp,udp)
- --quiet, -q             Suppress interactive prompts
- --debug, -g             Emit diagnostics output on error

xp2p client dns-forward remove
Options:
Includes: Global options
- --domain, -d <name>     Domain name to remove
- --with-forward, -W      Deprecated; auto-created target forwards are removed when unused
- --intercept, -I         Remove DNS intercept redirect
- --all, -a               Remove all managed DNS forward entries
- --quiet, -q             Suppress interactive prompts
- --debug, -g             Emit diagnostics output on error

Behavior:
- Removes the managed dnsmasq domain entry.
- Removes the xray forward only when it is owned by dns-forward and no remaining dns-forward entry uses the same listen port.
- Leaves pre-existing or externally owned forwards in place.

xp2p client dns-forward list
Options:
Includes: Global options
- --debug, -g             Emit diagnostics output on error
