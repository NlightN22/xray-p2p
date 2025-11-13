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
	// UnixConfigRoot is the default configuration root on Linux/OpenWrt.
	UnixConfigRoot = "/etc/xp2p"
	// UnixLogRoot is the default log root on Linux/OpenWrt.
	UnixLogRoot = "/var/log/xp2p"
)
