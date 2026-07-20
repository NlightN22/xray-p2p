# xp2p client

## Command tree

xp2p client
  Summary: Manage xp2p client installation
  Subcommands: install, disable, enable, update, remove, list, run, service, state, obs, render, debug, export, import, deploy, redirect, forward, reverse, mode, group, subscription, dns-forward
Options:
Includes: inherited options

xp2p client install
  Summary: Install xp2p client assets and reverse bridges
Options:
Includes: inherited options
- --allow-insecure, -I allow insecure TLS (skip verification)
- --config-dir, -D <dir> client configuration directory name
- --force, -f replace existing endpoint configuration
- --host, -H <host> remote server host
- --link, -L <link> client connection link
- --mode, -M <mode> target client mode (proxy or tun; also supports tun:split or tun:full)
- --password, -w <password> user password
- --path, -p <path> client installation directory
- --port, -P <port> remote server port
- --sni, -s <host> TLS server name (SNI)
- --strict-tls, -S enforce TLS verification
- --tun-mode, -m <mode> TUN routing mode (split or full)
- --user, -u <id> user email (used to derive the <user><host>.rev reverse bridge)

xp2p client disable [hostname|tag]
  Summary: Disable a client endpoint
Options:
Includes: inherited options
- --all, -a enable or disable all endpoints

xp2p client enable [hostname|tag]
  Summary: Enable a client endpoint
Options:
Includes: inherited options
- --all, -a enable or disable all endpoints

xp2p client update <hostname|tag>
  Summary: Update endpoint credentials
Options:
Includes: inherited options
- --config-dir, -D <dir> client configuration directory name
- --password, -w <password> user password
- --path, -p <path> client installation directory
- --user, -u <id> user email

xp2p client remove [hostname|tag]
  Summary: Remove xp2p client endpoints or entire installation
Options:
Includes: inherited options
- --all, -a remove all endpoints and configuration
- --config-dir, -D <dir> client configuration directory name
- --ignore-missing, -m do not fail if installation is absent
- --keep-files, -K keep installation files
- --path, -p <path> client installation directory
- --quiet, -q do not prompt for removal

xp2p client list
  Summary: List configured xp2p client endpoints
Options:
Includes: inherited options
- --config-dir, -D <dir> client configuration directory name
- --link, -L print client connection links
- --path, -p <path> client installation directory
- --pending, -y list pending configuration

xp2p client run
  Summary: Run xp2p client in foreground
Options:
Includes: inherited options
- --auto-install, -A install automatically if missing
- --config-dir, -D <dir> client configuration directory name
- --path, -p <path> client installation directory
- --quiet, -q do not prompt for installation
- --verbose, -V emit full-tunnel change details

xp2p client service
  Summary: Manage the xp2p client service
  Subcommands: start, stop, restart, status, run
Options:
Includes: inherited options

xp2p client service start
  Summary: Start the xp2p client service
Options:
Includes: inherited options

xp2p client service stop
  Summary: Stop the xp2p client service
Options:
Includes: inherited options

xp2p client service restart
  Summary: Restart the xp2p client service
Options:
Includes: inherited options

xp2p client service status
  Summary: Show xp2p client service status
Options:
Includes: inherited options

xp2p client service run
  Summary: Run the xp2p client service in the foreground
Options:
Includes: inherited options
- --config-dir, -D <dir> client configuration directory name
- --heartbeat, -b enable heartbeat probes
- --heartbeat-interval, -I <duration> heartbeat interval
- --heartbeat-port, -P <string> diagnostics service port to probe
- --heartbeat-socks, -S <string> SOCKS5 proxy for heartbeat (optional)
- --heartbeat-timeout, -T <duration> heartbeat timeout
- --log-file, -F <path> xp2p service log file (default: platform-specific path)
- --max-restarts, -R <n> maximum restart attempts after failures
- --path, -p <path> client installation directory
- --restart-delay, -r <duration> delay between restart attempts
- --verbose, -V emit full-tunnel change details

xp2p client state
  Summary: Show local heartbeat cache status
Options:
Includes: inherited options
- --interval, -i <duration> refresh interval for --watch
- --path, -p <path> client installation directory
- --pending, -y show pending configuration
- --ttl, -T <duration> heartbeat TTL for alive status
- --watch, -w continuously refresh state until interrupted
- --xray-api, -A <host:port> Xray API address for stats
- --xray-bin, -B <path> deprecated; stats use direct Xray gRPC
- --xray-stats, -X show Xray user traffic counters
- --xray-stats-format, -F <mode> Xray stats format (human|bytes)

xp2p client obs
  Summary: Show Xray outbound observations
Options:
Includes: inherited options
- --path, -p <path> client installation directory
- --xray-api, -A <host:port> Xray API address

xp2p client render
  Summary: Render compiled runtime artifacts
  Subcommands: xray
Options:
Includes: inherited options

xp2p client render xray
  Summary: Render xray.json
Options:
Includes: inherited options
- --desired, -d compile Desired inputs without applying
- --live, -L render live runtime artifacts
- --output, -o <path> output path ('-' for stdout)

xp2p client debug
  Summary: Debug helpers
  Subcommands: bundle
Options:
Includes: inherited options

xp2p client debug bundle
  Summary: Create a debug bundle archive
Options:
Includes: inherited options
- --output, -o <path> archive output path

xp2p client export
  Summary: Export client configuration bundle
Options:
Includes: inherited options
- --config-root, -C <dir> configuration root to export
- --output, -o <path> archive output path

xp2p client import
  Summary: Import client configuration bundle
Options:
Includes: inherited options
- --config-root, -C <dir> configuration root to import into
- --input, -i <path> (required) archive input path

xp2p client deploy
  Summary: Deploy xp2p client via remote helper
Options:
Includes: inherited options
- --force, -f allow changing existing tun mode
- --host, -H <host> (required) remote host (IP or DNS) to deploy
- --install-dir, -I <dir> server install directory override
- --mode, -M <mode> target client mode (proxy or tun; also supports tun:split or tun:full)
- --password, -w <password> user password (auto-generated when omitted)
- --port, -P <port> deploy port
- --trojan-port, -T <port> service port
- --tun-mode, -m <mode> TUN routing mode (split or full)
- --user, -u <id> user identifier (email)

xp2p client redirect
  Summary: Manage custom client redirects
  Default behavior: list configured redirect rules
  Subcommands: add, disable, enable, remove, list
Options:
Includes: inherited options
- --config-dir, -D <dir> client configuration directory name
- --path, -p <path> client installation directory
- --pending, -y list pending configuration

xp2p client redirect add
  Summary: Add a custom redirect rule
Options:
Includes: inherited options
- --cidr, -C <cidr> CIDR to redirect
- --config-dir, -D <dir> client configuration directory name
- --domain, -d <host> domain to redirect
- --host, -H <host> client endpoint hostname to route through
- --no-routes, -N do not add OS routes for CIDR redirects
- --path, -p <path> client installation directory
- --quiet, -q do not prompt for outbound tags
- --tag, -g <id> outbound tag to route through (prompts when omitted)

xp2p client redirect disable
  Summary: Disable a redirect rule
Options:
Includes: inherited options
- --all, -a toggle all redirect rules
- --cidr, -C <cidr> CIDR mapping to toggle
- --domain, -d <host> domain mapping to toggle
- --host, -H <host> client endpoint hostname filter
- --quiet, -q do not prompt for outbound tags
- --tag, -g <id> outbound tag filter

xp2p client redirect enable
  Summary: Enable a redirect rule
Options:
Includes: inherited options
- --all, -a toggle all redirect rules
- --cidr, -C <cidr> CIDR mapping to toggle
- --domain, -d <host> domain mapping to toggle
- --host, -H <host> client endpoint hostname filter
- --quiet, -q do not prompt for outbound tags
- --tag, -g <id> outbound tag filter

xp2p client redirect remove
  Summary: Remove a redirect rule
Options:
Includes: inherited options
- --cidr, -C <cidr> CIDR mapping to remove
- --config-dir, -D <dir> client configuration directory name
- --domain, -d <host> domain mapping to remove
- --host, -H <host> client endpoint hostname filter
- --path, -p <path> client installation directory
- --quiet, -q do not prompt for outbound tags
- --tag, -g <id> outbound tag filter (prompts when omitted)

xp2p client redirect list
  Summary: List configured redirect rules
Options:
Includes: inherited options
- --config-dir, -D <dir> client configuration directory name
- --path, -p <path> client installation directory
- --pending, -y list pending configuration

xp2p client forward
  Summary: Manage client dokodemo-door forwards
  Subcommands: add, remove, list
Options:
Includes: inherited options

xp2p client forward add
  Summary: Add a client dokodemo-door forward
Options:
Includes: inherited options
- --base-port, -B <port> first port to probe when auto-selecting
- --config-dir, -D <dir> client configuration directory name
- --listen, -n <host:port> local listen address (default 127.0.0.1)
- --listen-port, -P <port> local listen port (auto-select when omitted)
- --path, -p <path> client installation directory
- --proto, -o <proto> protocol to forward (tcp, udp, both)
- --target, -t <host:port> (required) target host:port to forward traffic to

xp2p client forward remove
  Summary: Remove a client dokodemo-door forward
Options:
Includes: inherited options
- --cleanup, -C remove state entry even when config is missing
- --config-dir, -D <dir> client configuration directory name
- --ignore-missing, -m do not fail when the forward rule does not exist
- --listen-port, -P <port> forward listen port
- --path, -p <path> client installation directory
- --remark, -r <id> forward remark
- --tag, -g <id> forward tag

xp2p client forward list
  Summary: List client forwards
Options:
Includes: inherited options
- --config-dir, -D <dir> client configuration directory name
- --path, -p <path> client installation directory
- --pending, -y list pending configuration

xp2p client reverse
  Summary: Inspect client reverse tunnels
  Default behavior: list client reverse tunnels
  Subcommands: disable, enable, list
Options:
Includes: inherited options
- --config-dir, -D <dir> client configuration directory name
- --path, -p <path> client installation directory
- --pending, -y list pending configuration

xp2p client reverse disable [tag|user|host]
  Summary: Disable a client reverse tunnel
Options:
Includes: inherited options
- --all, -a enable or disable all reverse tunnels

xp2p client reverse enable [tag|user|host]
  Summary: Enable a client reverse tunnel
Options:
Includes: inherited options
- --all, -a enable or disable all reverse tunnels

xp2p client reverse list
  Summary: List client reverse tunnels
Options:
Includes: inherited options
- --config-dir, -D <dir> client configuration directory name
- --path, -p <path> client installation directory
- --pending, -y list pending configuration

xp2p client mode [tun|proxy] [split|full]
  Summary: Switch client mode between TUN and proxy (optional tun mode)
Options:
Includes: inherited options
- --config-dir, -D <dir> client configuration directory name
- --host, -H <host> client endpoint hostname for full-tunnel routing
- --path, -p <path> client installation directory
- --quiet, -q do not prompt for outbound tags
- --tag, -g <id> outbound tag for full-tunnel routing (prompts when omitted)
- --verbose, -V emit full-tunnel change details

xp2p client group
  Summary: Inspect HA endpoint groups
  Subcommands: list
Options:
Includes: inherited options

xp2p client group list
  Summary: List HA endpoint groups
  Aliases: status, inspect
Options:
Includes: inherited options

xp2p client subscription
  Summary: Manage external server-authoritative subscriptions
  Subcommands: add, status, offers, refresh, remove
Options:
Includes: inherited options

xp2p client subscription add <id> <url>
  Summary: Add and fetch an external subscription
Options:
Includes: inherited options
- --allow-http, -A allow HTTP for a local compatibility fixture

xp2p client subscription status
  Summary: Show external subscription status
Options:
Includes: inherited options

xp2p client subscription offers
  Summary: List external connection offers
Options:
Includes: inherited options

xp2p client subscription refresh <id>
  Summary: Refresh one external subscription
Options:
Includes: inherited options
- --allow-http, -A allow HTTP for a local compatibility fixture

xp2p client subscription remove <id>
  Summary: Remove one external subscription and its offers
Options:
Includes: inherited options

xp2p client dns-forward
  Summary: Manage dnsmasq forward entries on OpenWrt
  Subcommands: add, remove, list
Options:
Includes: inherited options

xp2p client dns-forward add
  Summary: Create or update a DNS forward entry
Options:
Includes: inherited options
- --debug, -g emit diagnostics output on error
- --domain, -d <host> (required) domain name to match
- --intercept, -I install DNS intercept redirect (53/tcp,udp)
- --quiet, -q suppress interactive prompts
- --target, -t <host:port> (required) upstream DNS server (IP:port)
- --with-forward, -W deprecated; dns-forward always ensures a target forward

xp2p client dns-forward remove
  Summary: Remove a DNS forward entry
Options:
Includes: inherited options
- --all, -a remove all managed DNS forward entries
- --debug, -g emit diagnostics output on error
- --domain, -d <host> domain name to remove
- --intercept, -I remove DNS intercept redirect
- --quiet, -q suppress interactive prompts
- --with-forward, -W deprecated; auto-created target forwards are removed when unused

xp2p client dns-forward list
  Summary: List managed DNS forwards
Options:
Includes: inherited options
- --debug, -g emit diagnostics output on error

