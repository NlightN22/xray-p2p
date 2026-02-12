package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/configio"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/pelletier/go-toml"
)

// UpdateTunEnabled updates tun_enabled in the specified config file and returns the path used.
func UpdateTunEnabled(path string, role string, enabled bool) (string, error) {
	trimmedRole := strings.ToLower(strings.TrimSpace(role))
	if trimmedRole != "client" && trimmedRole != "server" {
		return "", fmt.Errorf("config: unsupported role %q", role)
	}

	configPath, err := resolveConfigPath(path, trimmedRole)
	if err != nil {
		return "", err
	}
	if strings.ToLower(filepath.Ext(configPath)) != ".toml" {
		return "", fmt.Errorf("config: only toml files are supported for mode changes")
	}

	tree, err := loadOrCreateToml(configPath)
	if err != nil {
		return "", err
	}
	tree.SetPath([]string{trimmedRole, "tun_enabled"}, enabled)

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

// EnsureTunSettings stores tun defaults in the role config when missing.
func EnsureTunSettings(path string, role string, enabled bool, name string, mtu int, addr string) (string, error) {
	trimmedRole := strings.ToLower(strings.TrimSpace(role))
	if trimmedRole != "client" && trimmedRole != "server" {
		return "", fmt.Errorf("config: unsupported role %q", role)
	}

	configPath, err := resolveConfigPath(path, trimmedRole)
	if err != nil {
		return "", err
	}
	if strings.ToLower(filepath.Ext(configPath)) != ".toml" {
		return "", fmt.Errorf("config: only toml files are supported for tun settings")
	}

	tree, err := loadOrCreateToml(configPath)
	if err != nil {
		return "", err
	}

	changed := false
	if tree.GetPath([]string{trimmedRole, "tun_enabled"}) == nil {
		tree.SetPath([]string{trimmedRole, "tun_enabled"}, enabled)
		changed = true
	}
	if tree.GetPath([]string{trimmedRole, "tun_name"}) == nil {
		tree.SetPath([]string{trimmedRole, "tun_name"}, strings.TrimSpace(name))
		changed = true
	}
	if tree.GetPath([]string{trimmedRole, "tun_mtu"}) == nil {
		tree.SetPath([]string{trimmedRole, "tun_mtu"}, mtu)
		changed = true
	}
	if tree.GetPath([]string{trimmedRole, "tun_addr"}) == nil {
		tree.SetPath([]string{trimmedRole, "tun_addr"}, strings.TrimSpace(addr))
		changed = true
	}

	if !changed {
		return configPath, nil
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

// UpdateServerTrojanPort updates server.trojan_port in the specified config file and returns the path used.
func UpdateServerTrojanPort(path string, port string) (string, error) {
	return updateServerTrojanPort(path, port, configio.WriteOptions{
		AuditPath: AuditLogPath(),
	})
}

// UpdateServerTrojanPortBestEffort updates server.trojan_port and ignores audit log errors.
func UpdateServerTrojanPortBestEffort(path string, port string) (string, error) {
	return updateServerTrojanPort(path, port, configio.WriteOptions{
		AuditPath:         AuditLogPath(),
		IgnoreAuditErrors: true,
	})
}

func updateServerTrojanPort(path string, port string, opts configio.WriteOptions) (string, error) {
	port = strings.TrimSpace(port)
	if port == "" {
		return "", fmt.Errorf("config: server trojan port is required")
	}
	portValue, err := strconv.Atoi(port)
	if err != nil || portValue <= 0 || portValue > 65535 {
		return "", fmt.Errorf("config: invalid server trojan port %q", port)
	}

	configPath, err := resolveConfigPath(path, "server")
	if err != nil {
		return "", err
	}
	if strings.ToLower(filepath.Ext(configPath)) != ".toml" {
		return "", fmt.Errorf("config: only toml files are supported for server trojan port updates")
	}

	tree, err := loadOrCreateToml(configPath)
	if err != nil {
		return "", err
	}
	tree.SetPath([]string{"server", "trojan_port"}, port)

	data, err := encodeToml(tree)
	if err != nil {
		return "", err
	}
	if err := configio.WriteBytes(configPath, data, opts); err != nil {
		return "", err
	}
	return configPath, nil
}

func resolveConfigPath(explicit, role string) (string, error) {
	if trimmed := strings.TrimSpace(explicit); trimmed != "" {
		return filepath.Clean(trimmed), nil
	}
	if strings.EqualFold(strings.TrimSpace(role), "client") {
		return filepath.Clean(ConfigPath(layout.ClientConfigFileName)), nil
	}
	if strings.EqualFold(strings.TrimSpace(role), "server") {
		return filepath.Clean(ConfigPath(layout.ServerConfigFileName)), nil
	}
	return "", fmt.Errorf("config: unsupported role %q", role)
}

func loadOrCreateToml(path string) (*toml.Tree, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			tree, err := toml.TreeFromMap(map[string]any{})
			if err != nil {
				return nil, fmt.Errorf("config: create empty toml tree: %w", err)
			}
			return tree, nil
		}
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	if strings.TrimSpace(string(data)) == "" {
		tree, err := toml.TreeFromMap(map[string]any{})
		if err != nil {
			return nil, fmt.Errorf("config: create empty toml tree: %w", err)
		}
		return tree, nil
	}
	tree, err := toml.LoadBytes(data)
	if err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return tree, nil
}

func encodeToml(tree *toml.Tree) ([]byte, error) {
	if tree == nil {
		return nil, errors.New("config: toml tree is nil")
	}
	data, err := toml.Marshal(tree.ToMap())
	if err != nil {
		return nil, fmt.Errorf("config: encode toml: %w", err)
	}
	return data, nil
}
