# xp2p server

## Command tree

xp2p server
  Summary: Manage xp2p server components
  Subcommands: install, remove, run, service, state, render, debug, export, import, user, identity, redirect, forward, reverse, cert, deploy, mode, profile, ha, dns-forward
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
- --profile, -r <string> server tunnel profile

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
  Subcommands: add, update, rotate, disable, enable, remove, list
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
- --link, -L <link> client connection link
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

xp2p server user rotate <id>
  Summary: Rotate a user credential
Options:
Includes: inherited options
- --ttl, -T <duration> previous credential rotation window

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

xp2p server identity
  Summary: Manage identity cache operations
  Subcommands: sync, status, provision, detach, select
Options:
Includes: inherited options

xp2p server identity sync
  Summary: Synchronize identity cache
Options:
Includes: inherited options

xp2p server identity status
  Summary: Show identity cache status
Options:
Includes: inherited options

xp2p server identity provision <label>
  Summary: Provision a cached identity as a server user
Options:
Includes: inherited options
- --host, -H <host> public host name or IP for generated connection link

xp2p server identity detach
  Summary: Detach the selected identity provider
Options:
Includes: inherited options

xp2p server identity select <instance-id>
  Summary: Select or reattach an identity provider
Options:
Includes: inherited options
- --group, -G <stringSlice> provider group scope
- --kind, -K <string> (required) provider kind: ldap or scim

xp2p server redirect
  Summary: Manage server redirect rules
  Default behavior: list server redirect rules
  Subcommands: add, disable, enable, remove, list, access
Options:
Includes: inherited options
- --config-dir, -D <dir> server configuration directory name or absolute path
- --path, -p <path> server installation directory
- --pending, -y list pending configuration

xp2p server redirect add
  Summary: Add a server redirect rule
Options:
Includes: inherited options
- --access, -V <string> access policy: all or restricted
- --allow-group, -G <stringSlice> allowed provider group ID (repeatable)
- --allow-user, -U <stringSlice> allowed user label (repeatable)
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

xp2p server redirect access
  Summary: Manage redirect access policies
  Subcommands: set, add-user, remove-user, add-group, remove-group, clear
Options:
Includes: inherited options

xp2p server redirect access set
  Summary: Replace a redirect access policy
Options:
Includes: inherited options
- --access, -V <string> access policy: all or restricted
- --allow-group, -G <stringSlice> allowed provider group ID (repeatable)
- --allow-user, -U <stringSlice> allowed user label (repeatable)
- --cidr, -C <cidr> CIDR redirect selector
- --domain, -d <host> domain redirect selector
- --host, -H <host> reverse portal host
- --tag, -g <id> reverse outbound tag

xp2p server redirect access add-user
  Summary: Add allowed users
Options:
Includes: inherited options
- --allow-user, -U <stringSlice> allowed user label (repeatable)
- --cidr, -C <cidr> CIDR redirect selector
- --domain, -d <host> domain redirect selector
- --host, -H <host> reverse portal host
- --tag, -g <id> reverse outbound tag

xp2p server redirect access remove-user
  Summary: Remove allowed users
Options:
Includes: inherited options
- --allow-user, -U <stringSlice> allowed user label (repeatable)
- --cidr, -C <cidr> CIDR redirect selector
- --domain, -d <host> domain redirect selector
- --host, -H <host> reverse portal host
- --tag, -g <id> reverse outbound tag

xp2p server redirect access add-group
  Summary: Add allowed groups
Options:
Includes: inherited options
- --allow-group, -G <stringSlice> allowed provider group ID (repeatable)
- --cidr, -C <cidr> CIDR redirect selector
- --domain, -d <host> domain redirect selector
- --host, -H <host> reverse portal host
- --tag, -g <id> reverse outbound tag

xp2p server redirect access remove-group
  Summary: Remove allowed groups
Options:
Includes: inherited options
- --allow-group, -G <stringSlice> allowed provider group ID (repeatable)
- --cidr, -C <cidr> CIDR redirect selector
- --domain, -d <host> domain redirect selector
- --host, -H <host> reverse portal host
- --tag, -g <id> reverse outbound tag

xp2p server redirect access clear
  Summary: Clear redirect access selectors
Options:
Includes: inherited options
- --cidr, -C <cidr> CIDR redirect selector
- --domain, -d <host> domain redirect selector
- --host, -H <host> reverse portal host
- --tag, -g <id> reverse outbound tag

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
  Default behavior: list server reverse tunnels
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
- --quiet, -q suppress interactive prompts

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

xp2p server profile [trojan-tls|vless-tls-vision]
  Summary: Show or switch the server tunnel profile
Options:
Includes: inherited options

xp2p server ha
  Summary: Manage server HA topology
  Subcommands: status, group, sync, peer, channel, redirect, member
Options:
Includes: inherited options

xp2p server ha status
  Summary: Show committed HA generation
Options:
Includes: inherited options

xp2p server ha group
  Summary: Manage the HA group
  Subcommands: create, remove, inspect, update
Options:
Includes: inherited options

xp2p server ha group create <id> <tag>
  Summary: Create an HA group
Options:
Includes: inherited options

xp2p server ha group remove
  Summary: Remove an HA group after channel rebind or disable
Options:
Includes: inherited options

xp2p server ha group inspect
  Summary: Inspect HA group topology
Options:
Includes: inherited options

xp2p server ha group update <automatic|manual|disabled>
  Summary: Set HA group selector mode
Options:
Includes: inherited options

xp2p server ha sync
  Summary: Synchronize the next HA generation with peers
Options:
Includes: inherited options

xp2p server ha peer
  Summary: Manage trusted HA peers
  Subcommands: self, add, remove, list
Options:
Includes: inherited options

xp2p server ha peer self <id>
  Summary: Set the local HA peer identity
Options:
Includes: inherited options

xp2p server ha peer add <id> <endpoint> <secret>
  Summary: Add or update an HA peer
Options:
Includes: inherited options
- --allow-insecure, -k allow an untrusted peer certificate
- --non-voting, -n exclude peer from quorum voting
- --witness, -w mark peer as a control-plane witness

xp2p server ha peer remove <id>
  Summary: Remove an HA peer
Options:
Includes: inherited options

xp2p server ha peer list
  Summary: List HA peers
Options:
Includes: inherited options

xp2p server ha channel
  Summary: Manage stable HA reverse channels
  Subcommands: create, disable, inspect, rebind, rebind-endpoint, finalize, list
Options:
Includes: inherited options

xp2p server ha channel create <id> <tag> <domain>
  Summary: Create a group-bound HA channel
Options:
Includes: inherited options

xp2p server ha channel disable <id>
  Summary: Disable an HA channel
Options:
Includes: inherited options

xp2p server ha channel inspect <id>
  Summary: Inspect an HA channel
Options:
Includes: inherited options

xp2p server ha channel rebind <id> <group-tag|endpoint-tag>
  Summary: Rebind an HA channel
Options:
Includes: inherited options

xp2p server ha channel rebind-endpoint <id> <endpoint-tag>
  Summary: Bind an HA channel to a physical endpoint
Options:
Includes: inherited options

xp2p server ha channel finalize <id>
  Summary: Finalize a disabled HA channel
Options:
Includes: inherited options

xp2p server ha channel list
  Summary: List HA channels
Options:
Includes: inherited options

xp2p server ha redirect
  Summary: Manage group-owned HA redirect policy
  Subcommands: add, remove, list
Options:
Includes: inherited options

xp2p server ha redirect add <channel-id>
  Summary: Add a redirect through a group-bound HA channel
Options:
Includes: inherited options
- --access, -V <string> access policy: all or restricted
- --allow-group, -G <stringSlice> allowed provider group ID (repeatable)
- --allow-user, -U <stringSlice> allowed user label (repeatable)
- --cidr, -C <cidr> CIDR to redirect
- --domain, -d <host> domain to redirect

xp2p server ha redirect remove <channel-id>
  Summary: Remove a redirect through a group-bound HA channel
Options:
Includes: inherited options
- --cidr, -C <cidr> CIDR mapping to remove
- --domain, -d <host> domain mapping to remove

xp2p server ha redirect list
  Summary: List group-owned HA redirect policy
Options:
Includes: inherited options

xp2p server ha member
  Summary: Manage HA group members
  Subcommands: remove, add, reprioritize, list
Options:
Includes: inherited options

xp2p server ha member remove <id>
  Summary: Tombstone an HA member
Options:
Includes: inherited options
- --force, -f force an emergency two-voter reconfiguration
- --reason, -r <string> audit reason for emergency force-reconfiguration

xp2p server ha member add <id> <tag> <host> <port> <profile>
  Summary: Add a confirmed HA member
Options:
Includes: inherited options
- --force, -f force an emergency two-voter reconfiguration
- --reason, -r <string> audit reason for emergency force-reconfiguration
- --tls-pin, -P <string> pinned peer certificate SHA256 advertised for this HA member
- --tls-server-name, -S <string> TLS server name advertised for this HA member

xp2p server ha member reprioritize <id> <priority>
  Summary: Change HA member priority
Options:
Includes: inherited options

xp2p server ha member list
  Summary: List HA group members
Options:
Includes: inherited options

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

