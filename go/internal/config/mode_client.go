package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/configio"
)

type ClientModeUpdate struct {
	TunEnabled bool

	SetTunMode bool
	TunMode    string

	SetFullTunnelVerbose bool
	FullTunnelVerbose    bool

	SetFullTunnelTag bool
	FullTunnelTag    string
}

// UpdateClientMode applies multiple client mode settings in a single write.
func UpdateClientMode(path string, update ClientModeUpdate) (string, error) {
	configPath, err := resolveConfigPath(path, "client")
	if err != nil {
		return "", err
	}
	if strings.ToLower(filepath.Ext(configPath)) != ".toml" {
		return "", fmt.Errorf("config: only toml files are supported for mode changes")
	}

	tree, err := loadOrCreateModeToml(configPath, "client")
	if err != nil {
		return "", err
	}
	tree.SetPath([]string{"client", "tun_enabled"}, update.TunEnabled)
	if update.SetTunMode {
		normalized, err := parseTunMode(update.TunMode)
		if err != nil {
			return "", err
		}
		tree.SetPath([]string{"client", "tun_mode"}, normalized)
	}
	if update.SetFullTunnelVerbose {
		tree.SetPath([]string{"client", "full_tunnel_verbose"}, update.FullTunnelVerbose)
	}
	if update.SetFullTunnelTag {
		tree.SetPath([]string{"client", "full_tunnel_tag"}, strings.TrimSpace(update.FullTunnelTag))
	}

	data, err := encodeToml(tree)
	if err != nil {
		return "", err
	}
	if err := configio.WriteBytes(configPath, data, configio.WriteOptions{
		AuditPath: AuditLogPath(),
	}); err != nil {
		return "", err
	}
	return configPath, nil
}
