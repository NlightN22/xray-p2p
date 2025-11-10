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
	// StateFileName tracks installation metadata for xp2p.
	StateFileName = "install-state.json"
)
