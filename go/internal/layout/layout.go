package layout

// Common directory and file names that make up an xp2p installation.
const (
	// BinDirName stores auxiliary binaries (xray-core, helpers, etc.).
	BinDirName = "bin"
	// LogsDirName contains log files written by xp2p/xray-core.
	LogsDirName = "logs"
	// ClientConfigDir holds client-side configuration JSON files.
	ClientConfigDir = "config-client"
	// ServerConfigDir holds server-side configuration JSON files.
	ServerConfigDir = "config-server"
	// StateFileName tracks installation metadata for xp2p (legacy single-role marker).
	StateFileName = "install-state.json"
	// ClientStateFileName is the canonical client marker name.
	ClientStateFileName = "install-state-client.json"
	// ServerStateFileName is the canonical server marker name.
	ServerStateFileName = "install-state-server.json"
	// ClientConfigFileName stores client configuration in the default config root.
	ClientConfigFileName = "xp2p-client.toml"
	// ServerConfigFileName stores server configuration in the default config root.
	ServerConfigFileName = "xp2p-server.toml"
	// ClientAppliedStateFileName stores applied client state in the default config root.
	ClientAppliedStateFileName = "xp2p-client.state.json"
	// ServerAppliedStateFileName stores applied server state in the default config root.
	ServerAppliedStateFileName = "xp2p-server.state.json"
	// AuditLogFileName stores configuration change audit logs.
	AuditLogFileName = "xp2p-audit.log"
	// HeartbeatStateFileName is retained for legacy shared heartbeat storage.
	HeartbeatStateFileName = "state-heartbeat.json"
	// ClientHeartbeatStateFileName stores client-side heartbeat snapshots.
	ClientHeartbeatStateFileName = "state-heartbeat-client.json"
	// ServerHeartbeatStateFileName stores server-side heartbeat snapshots.
	ServerHeartbeatStateFileName = "state-heartbeat-server.json"
	// UnixConfigRoot is the default configuration root on Linux/OpenWrt.
	UnixConfigRoot = "/etc/xp2p"
	// UnixLogRoot is the default log root on Linux/OpenWrt.
	UnixLogRoot = "/var/log/xp2p"
)
