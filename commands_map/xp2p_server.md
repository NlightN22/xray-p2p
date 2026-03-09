# xp2p server

## Global options (apply to all commands)
- --help, -h                     Show help for command
- --config, -c <path>             Path to configuration file
- --log-level, -l <level>         Override logging level
- --log-json, -j                  Emit logs in JSON format
- --version, -v                   Print xp2p version and exit (root only)

## Command tree

xp2p server
  Subcommands: install, remove, run, service, state, export, import, user, redirect, forward, reverse, cert, deploy, mode, dns-forward (Linux only)

xp2p server install
Options:
Includes: Global options
- --path, -p <dir>       Server installation directory
- --config-dir, -D <dir> Server configuration directory name
- --port, -P <port>      Server listener port
- --cert-store, -S <ref> TLS certificate store reference (win-store)
- --cert, -E <path>      TLS certificate file to deploy
- --key, -k <path>       TLS private key file to deploy
- --host, -H <host>      Public host name or IP for generated configuration
- --force, -f            Overwrite existing installation

xp2p server remove
Options:
Includes: Global options
- --path, -p <dir>       Server installation directory
- --config-dir, -D <dir> Server configuration directory name
- --keep-files, -K       Keep installation files
- --ignore-missing, -m   Do not fail if service or files are absent
- --quiet, -q            Do not prompt for removal

xp2p server run
Options:
Includes: Global options
- --path, -p <dir>          Server installation directory
- --config-dir, -D <dir>    Server configuration directory name
- --diag-service-port, -P <port> Diagnostics service port
- --diag-service-mode, -M <auto|manual> Diagnostics service startup mode
- --auto-install, -A        Install server assets when missing without prompting
- --quiet, -q               Suppress interactive prompts
- --xray-log-file, -X <path> Append xray stderr output to file

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
- --path, -p <dir>       Server installation directory
- --config-dir, -D <dir> Server configuration directory name
- --diag-service-port, -P <port> Diagnostics service port
- --diag-service-mode, -M <auto|manual> Diagnostics service startup mode
- --log-file, -F <path>  xp2p service log file
- --xray-log-file, -X <path> xray stderr log file
- --max-restarts, -R <n> Maximum restart attempts after failures
- --restart-delay, -r <dur> Delay between restart attempts

xp2p server state
Options:
Includes: Global options
- --path, -p <dir>       Server installation directory
- --watch, -w            Continuously refresh state until interrupted
- --interval, -i <dur>   Refresh interval for --watch
- --ttl, -T <dur>        Heartbeat TTL for alive status

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
- --path, -p <dir>       Server installation directory
- --config-dir, -D <dir> Server configuration directory name or absolute path
- --id, -i <id>          Trojan client identifier (derives the <id><host>.rev reverse tag)
- --password, -w <password> Trojan client password or pre-shared key (auto-generated when omitted)
- --key, -k <password>   Alias for --password
- --host, -H <host>      Public host name or IP for generated connection link
- --no-reverse, -n       Skip creating reverse portal/routing entries
- --force, -f            Overwrite existing user entry

xp2p server user remove
Options:
Includes: Global options
- --path, -p <dir>       Server installation directory
- --config-dir, -D <dir> Server configuration directory name or absolute path
- --id, -i <id> (required) Trojan client identifier
- --host, -H <host>      Public host name or IP (defaults to server host)

xp2p server user list
Options:
Includes: Global options
- --path, -p <dir>       Server installation directory
- --config-dir, -D <dir> Server configuration directory name or absolute path
- --host, -H <host>      Public host name or IP for generated connection links

xp2p server redirect add
Options:
Includes: Global options
- --path, -p <dir>       Server installation directory
- --config-dir, -D <dir> Server configuration directory name or absolute path
- --cidr, -C <cidr>      CIDR to redirect (mutually exclusive with --domain)
- --domain, -d <name>    Domain to redirect (mutually exclusive with --cidr)
- --tag, -g <tag>        Reverse outbound tag to route through (prompts when omitted)
- --host, -H <host>      Reverse portal host to route through
- --quiet, -q            Do not prompt for outbound tags

xp2p server redirect remove
Options:
Includes: Global options
- --path, -p <dir>       Server installation directory
- --config-dir, -D <dir> Server configuration directory name or absolute path
- --cidr, -C <cidr>      CIDR mapping to remove (mutually exclusive with --domain)
- --domain, -d <name>    Domain mapping to remove (mutually exclusive with --cidr)
- --tag, -g <tag>        Reverse outbound tag filter (prompts when omitted)
- --host, -H <host>      Reverse portal host filter
- --quiet, -q            Do not prompt for outbound tags

xp2p server redirect list
Options:
Includes: Global options
- --path, -p <dir>       Server installation directory
- --config-dir, -D <dir> Server configuration directory name or absolute path

xp2p server forward add
Options:
Includes: Global options
- --path, -p <dir>           Server installation directory
- --config-dir, -D <dir>     Server configuration directory name or absolute path
- --target, -t <host:port> (required) Target host:port to forward traffic to
- --listen, -n <host>        Local listen address (default 127.0.0.1)
- --listen-port, -P <n>      Local listen port (auto-select when omitted)
- --proto, -o <tcp|udp|both> Protocol to forward
- --base-port, -B <n>        First port to probe when auto-selecting

xp2p server forward remove
Options:
Includes: Global options
- --path, -p <dir>       Server installation directory
- --config-dir, -D <dir> Server configuration directory name or absolute path
- --listen-port, -P <n>  Forward listen port
- --tag, -g <tag>        Forward tag filter
- --remark, -r <text>    Forward remark filter
- --ignore-missing, -m   Do not fail when the forward rule does not exist

xp2p server forward list
Options:
Includes: Global options
- --path, -p <dir>       Server installation directory
- --config-dir, -D <dir> Server configuration directory name or absolute path

xp2p server reverse
Options:
Includes: Global options
- --path, -p <dir>       Server installation directory
- --config-dir, -D <dir> Server configuration directory name or absolute path

xp2p server reverse list
Options:
Includes: Global options
- --path, -p <dir>       Server installation directory
- --config-dir, -D <dir> Server configuration directory name or absolute path

xp2p server cert set
Options:
Includes: Global options
- --path, -p <dir>       Server installation directory
- --config-dir, -D <dir> Server configuration directory name or absolute path
- --cert-store, -S <ref> TLS certificate store reference (win-store)
- --cert, -E <path>      TLS certificate file to deploy
- --key, -k <path>       TLS private key file to deploy
- --host, -H <host>      Public host name or IP for certificate generation
- --force, -f            Overwrite existing TLS configuration without prompting

xp2p server cert state
Options:
Includes: Global options
- --path, -p <dir>       Server installation directory
- --config-dir, -D <dir> Server configuration directory name or absolute path

xp2p server deploy
Options:
Includes: Global options
- --listen, -n <host:port> Deploy listen address (default :62025)
- --link, -L <trojan://...> (required) Deploy link
- --timeout, -t <dur> Idle shutdown timeout

xp2p server mode [tun|proxy]
Options:
Includes: Global options
- --path, -p <dir>       Server installation directory
- --config-dir, -D <dir> Server configuration directory name
- --config, -c <path>    Path to configuration file (toml)

xp2p server dns-forward (Linux only)
  Subcommands: add, remove, list

xp2p server dns-forward add
Options:
Includes: Global options
- --domain, -d <name> (required) Domain name to match
- --target, -t <ip:port> (required) Upstream DNS server
- --with-forward, -W      Create or reuse a port forward for the target
- --intercept, -I         Install DNS intercept redirect (53/tcp,udp)
- --quiet, -q             Suppress interactive prompts
- --debug, -g             Emit diagnostics output on error

xp2p server dns-forward remove
Options:
Includes: Global options
- --domain, -d <name>     Domain name to remove
- --with-forward, -W      Remove an auto-created port forward
- --intercept, -I         Remove DNS intercept redirect
- --all, -a               Remove all managed DNS forward entries
- --quiet, -q             Suppress interactive prompts
- --debug, -g             Emit diagnostics output on error

xp2p server dns-forward list
Options:
Includes: Global options
- --debug, -g             Emit diagnostics output on error
