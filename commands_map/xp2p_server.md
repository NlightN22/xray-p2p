# xp2p server

## Command tree

xp2p server
  Summary: Manage xp2p server components
  Subcommands: install, remove, run, service, state, render, debug, export, import, user, redirect, forward, reverse, cert, deploy, mode, dns-forward
Options:
Includes: inherited options

xp2p server install
  Summary: Install xp2p server assets
Options:
Includes: inherited options
- --cert, -E <path> TLS certificate file to deploy
- --cert-store, -S <ref> TLS certificate store reference (win-store)
- --config-dir, -D <dir> server configuration directory name
- --force, -f overwrite existing installation
- --host, -H <host> public host name or IP for generated configuration
- --key, -k <password> TLS private key file to deploy
- --path, -p <path> server installation directory
- --port, -P <port> server listener port

xp2p server remove
  Summary: Remove xp2p server installation
Options:
Includes: inherited options
- --config-dir, -D <dir> server configuration directory name
- --ignore-missing, -m do not fail if service or files are absent
- --keep-files, -K keep installation files
- --path, -p <path> server installation directory
- --quiet, -q do not prompt for removal

xp2p server run
  Summary: Run xp2p server in foreground
Options:
Includes: inherited options
- --auto-install, -A install server assets when missing without prompting
- --config-dir, -D <dir> server configuration directory name
- --diag-service-mode, -M <mode> diagnostics service startup mode (auto|manual)
- --diag-service-port, -P <port> diagnostics service port
- --path, -p <path> server installation directory
- --quiet, -q suppress interactive prompts

xp2p server service
  Summary: Manage the xp2p server service
  Subcommands: start, stop, restart, status, run
Options:
Includes: inherited options

xp2p server service start
  Summary: Start the xp2p server service
Options:
Includes: inherited options

xp2p server service stop
  Summary: Stop the xp2p server service
Options:
Includes: inherited options

xp2p server service restart
  Summary: Restart the xp2p server service
Options:
Includes: inherited options

xp2p server service status
  Summary: Show xp2p server service status
Options:
Includes: inherited options

xp2p server service run
  Summary: Run the xp2p server service in the foreground
Options:
Includes: inherited options
- --config-dir, -D <dir> server configuration directory name
- --diag-service-mode, -M <mode> diagnostics service startup mode (auto|manual)
- --diag-service-port, -P <port> diagnostics service port
- --log-file, -F <path> xp2p service log file
- --max-restarts, -R <n> maximum restart attempts after failures
- --path, -p <path> server installation directory
- --restart-delay, -r <duration> delay between restart attempts

xp2p server state
  Summary: Show heartbeat status for xp2p tunnels
Options:
Includes: inherited options
- --interval, -i <duration> refresh interval for --watch
- --path, -p <path> server installation directory
- --pending, -y show pending configuration
- --ttl, -T <duration> heartbeat TTL for alive status
- --watch, -w continuously refresh state until interrupted
- --xray-api, -A <host:port> Xray API address for stats
- --xray-bin, -B <path> deprecated; stats use direct Xray gRPC
- --xray-stats, -X show Xray user traffic counters
- --xray-stats-format, -F <mode> Xray stats format (human|bytes)

xp2p server render
  Summary: Render compiled runtime artifacts
  Subcommands: xray
Options:
Includes: inherited options

xp2p server render xray
  Summary: Render xray.json
Options:
Includes: inherited options
- --desired, -d compile Desired inputs without applying
- --live, -L render live runtime artifacts
- --output, -o <path> output path ('-' for stdout)

xp2p server debug
  Summary: Debug helpers
  Subcommands: bundle
Options:
Includes: inherited options

xp2p server debug bundle
  Summary: Create a debug bundle archive
Options:
Includes: inherited options
- --output, -o <path> archive output path

xp2p server export
  Summary: Export server configuration bundle
Options:
Includes: inherited options
- --config-root, -C <dir> configuration root to export
- --output, -o <path> archive output path

xp2p server import
  Summary: Import server configuration bundle
Options:
Includes: inherited options
- --config-root, -C <dir> configuration root to import into
- --input, -i <path> (required) archive input path

xp2p server user
  Summary: Manage users on the server
  Subcommands: add, update, disable, enable, remove, list
Options:
Includes: inherited options

xp2p server user add
  Summary: Add a user and reverse portal
Options:
Includes: inherited options
- --config-dir, -D <dir> server configuration directory name or absolute path
- --force, -f overwrite existing user entry
- --host, -H <host> public host name or IP for generated connection link
- --id, -i <id> client identifier (derives the <id><host>.rev reverse tag)
- --key, -k <password> alias for --password
- --no-reverse, -n skip creating reverse portal/routing entries
- --password, -w <password> client password or pre-shared key (auto-generated when omitted)
- --path, -p <path> server installation directory

xp2p server user update <id>
  Summary: Update user credentials
Options:
Includes: inherited options
- --config-dir, -D <dir> server configuration directory name or absolute path
- --new-id, -I <id> new client identifier
- --password, -w <password> client password or pre-shared key
- --path, -p <path> server installation directory

xp2p server user disable <id>
  Summary: Disable a user
Options:
Includes: inherited options
- --all, -a enable or disable all users

xp2p server user enable <id>
  Summary: Enable a user
Options:
Includes: inherited options
- --all, -a enable or disable all users

xp2p server user remove
  Summary: Remove a user
Options:
Includes: inherited options
- --config-dir, -D <dir> server configuration directory name or absolute path
- --host, -H <host> public host name or IP (defaults to server host)
- --id, -i <id> (required) client identifier
- --path, -p <path> server installation directory

xp2p server user list
  Summary: List configured users
Options:
Includes: inherited options
- --config-dir, -D <dir> server configuration directory name or absolute path
- --host, -H <host> public host name or IP for generated connection links
- --path, -p <path> server installation directory
- --pending, -y list pending configuration

xp2p server redirect
  Summary: Manage server redirect rules
  Subcommands: add, disable, enable, remove, list
Options:
Includes: inherited options

xp2p server redirect add
  Summary: Add a server redirect rule
Options:
Includes: inherited options
- --cidr, -C <cidr> CIDR to redirect
- --config-dir, -D <dir> server configuration directory name or absolute path
- --domain, -d <host> domain to redirect
- --host, -H <host> reverse portal host to route through
- --no-routes, -N do not add OS routes for CIDR redirects
- --path, -p <path> server installation directory
- --quiet, -q do not prompt for outbound tags
- --tag, -g <id> reverse outbound tag to route through (prompts when omitted)
- --user, -u <id> reverse user to route through

xp2p server redirect disable
  Summary: Disable a server redirect rule
Options:
Includes: inherited options
- --all, -a toggle all redirect rules
- --cidr, -C <cidr> CIDR mapping to toggle
- --domain, -d <host> domain mapping to toggle
- --host, -H <host> reverse portal host filter
- --quiet, -q do not prompt for outbound tags
- --tag, -g <id> reverse outbound tag filter

xp2p server redirect enable
  Summary: Enable a server redirect rule
Options:
Includes: inherited options
- --all, -a toggle all redirect rules
- --cidr, -C <cidr> CIDR mapping to toggle
- --domain, -d <host> domain mapping to toggle
- --host, -H <host> reverse portal host filter
- --quiet, -q do not prompt for outbound tags
- --tag, -g <id> reverse outbound tag filter

xp2p server redirect remove
  Summary: Remove a server redirect rule
Options:
Includes: inherited options
- --cidr, -C <cidr> CIDR mapping to remove
- --config-dir, -D <dir> server configuration directory name or absolute path
- --domain, -d <host> domain mapping to remove
- --host, -H <host> reverse portal host filter
- --path, -p <path> server installation directory
- --quiet, -q do not prompt for outbound tags
- --tag, -g <id> reverse outbound tag filter or tag-only cleanup selector

xp2p server redirect list
  Summary: List server redirect rules
Options:
Includes: inherited options
- --config-dir, -D <dir> server configuration directory name or absolute path
- --path, -p <path> server installation directory
- --pending, -y list pending configuration

xp2p server forward
  Summary: Manage server dokodemo-door forwards
  Subcommands: add, remove, list
Options:
Includes: inherited options

xp2p server forward add
  Summary: Add a server dokodemo-door forward
Options:
Includes: inherited options
- --base-port, -B <port> first port to probe when auto-selecting
- --config-dir, -D <dir> server configuration directory name or absolute path
- --listen, -n <host:port> local listen address (default 127.0.0.1)
- --listen-port, -P <port> local listen port (auto-select when omitted)
- --path, -p <path> server installation directory
- --proto, -o <proto> protocol to forward (tcp, udp, both)
- --target, -t <host:port> (required) target host:port to forward traffic to

xp2p server forward remove
  Summary: Remove a server forward
Options:
Includes: inherited options
- --config-dir, -D <dir> server configuration directory name or absolute path
- --ignore-missing, -m do not fail when the forward rule does not exist
- --listen-port, -P <port> forward listen port
- --path, -p <path> server installation directory
- --remark, -r <id> forward remark
- --tag, -g <id> forward tag

xp2p server forward list
  Summary: List server forwards
Options:
Includes: inherited options
- --config-dir, -D <dir> server configuration directory name or absolute path
- --path, -p <path> server installation directory
- --pending, -y list pending configuration

xp2p server reverse
  Summary: Inspect server reverse tunnels
  Subcommands: disable, enable, list
Options:
Includes: inherited options
- --config-dir, -D <dir> server configuration directory name or absolute path
- --path, -p <path> server installation directory
- --pending, -y list pending configuration

xp2p server reverse disable [tag|user|host]
  Summary: Disable a server reverse tunnel
Options:
Includes: inherited options
- --all, -a enable or disable all reverse tunnels

xp2p server reverse enable [tag|user|host]
  Summary: Enable a server reverse tunnel
Options:
Includes: inherited options
- --all, -a enable or disable all reverse tunnels

xp2p server reverse list
  Summary: List server reverse tunnels
Options:
Includes: inherited options
- --config-dir, -D <dir> server configuration directory name or absolute path
- --path, -p <path> server installation directory
- --pending, -y list pending configuration

xp2p server cert
  Summary: Manage TLS certificates
  Subcommands: state, set
Options:
Includes: inherited options

xp2p server cert state
  Summary: Show TLS certificate status
Options:
Includes: inherited options
- --config-dir, -D <dir> server configuration directory name or absolute path
- --path, -p <path> server installation directory
- --pending, -y show pending configuration

xp2p server cert set
  Summary: Set or replace TLS certificates
Options:
Includes: inherited options
- --cert, -E <path> TLS certificate file to deploy
- --cert-store, -S <ref> TLS certificate store reference (win-store)
- --config-dir, -D <dir> server configuration directory name or absolute path
- --force, -f overwrite existing TLS configuration without prompting
- --host, -H <host> public host name or IP for certificate generation
- --key, -k <password> TLS private key file to deploy
- --path, -p <path> server installation directory

xp2p server deploy
  Summary: Listen for xp2p client deploy requests
Options:
Includes: inherited options
- --diag-service-port, -P <port> diagnostics service port
- --link, -L <link> (required) deploy link (trojan://...)
- --listen, -n <host:port> deploy listen address
- --timeout, -t <duration> idle shutdown timeout

xp2p server mode [tun|proxy]
  Summary: Switch server mode between TUN and proxy
Options:
Includes: inherited options
- --config-dir, -D <dir> server configuration directory name
- --path, -p <path> server installation directory

xp2p server dns-forward
  Summary: Manage dnsmasq forward entries on OpenWrt
  Subcommands: add, remove, list
Options:
Includes: inherited options

xp2p server dns-forward add
  Summary: Create or update a DNS forward entry
Options:
Includes: inherited options
- --debug, -g emit diagnostics output on error
- --domain, -d <host> (required) domain name to match
- --intercept, -I install DNS intercept redirect (53/tcp,udp)
- --quiet, -q suppress interactive prompts
- --target, -t <host:port> (required) upstream DNS server (IP:port)
- --with-forward, -W deprecated; dns-forward always ensures a target forward

xp2p server dns-forward remove
  Summary: Remove a DNS forward entry
Options:
Includes: inherited options
- --all, -a remove all managed DNS forward entries
- --debug, -g emit diagnostics output on error
- --domain, -d <host> domain name to remove
- --intercept, -I remove DNS intercept redirect
- --quiet, -q suppress interactive prompts
- --with-forward, -W deprecated; auto-created target forwards are removed when unused

xp2p server dns-forward list
  Summary: List managed DNS forwards
Options:
Includes: inherited options
- --debug, -g emit diagnostics output on error

