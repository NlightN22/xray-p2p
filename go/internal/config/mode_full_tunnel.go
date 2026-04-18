package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/configio"
)

// UpdateFullTunnelVerbose updates full_tunnel_verbose in the specified config file and returns the path used.
func UpdateFullTunnelVerbose(path string, enabled bool) (string, error) {
	trimmedRole := "client"
	configPath, err := resolveConfigPath(path, trimmedRole)
	if err != nil {
		return "", err
	}
	if strings.ToLower(filepath.Ext(configPath)) != ".toml" {
		return "", fmt.Errorf("config: only toml files are supported for verbose updates")
	}

	tree, err := loadOrCreateModeToml(configPath, trimmedRole)
	if err != nil {
		return "", err
	}
	tree.SetPath([]string{trimmedRole, "full_tunnel_verbose"}, enabled)

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

// UpdateFullTunnelTag updates full_tunnel_tag in the specified config file and returns the path used.
func UpdateFullTunnelTag(path string, tag string) (string, error) {
	trimmedRole := "client"
	configPath, err := resolveConfigPath(path, trimmedRole)
	if err != nil {
		return "", err
	}
	if strings.ToLower(filepath.Ext(configPath)) != ".toml" {
		return "", fmt.Errorf("config: only toml files are supported for full tunnel tag updates")
	}

	tree, err := loadOrCreateModeToml(configPath, trimmedRole)
	if err != nil {
		return "", err
	}
	tree.SetPath([]string{trimmedRole, "full_tunnel_tag"}, strings.TrimSpace(tag))

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
