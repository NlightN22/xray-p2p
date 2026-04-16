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

	tree, err := loadOrCreateModeToml(configPath, trimmedRole)
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

	tree, err := loadOrCreateModeToml(configPath, trimmedRole)
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

// UpdateTunMode updates tun_mode in the specified config file and returns the path used.
func UpdateTunMode(path string, role string, mode string) (string, error) {
	trimmedRole := strings.ToLower(strings.TrimSpace(role))
	if trimmedRole != "client" {
		return "", fmt.Errorf("config: unsupported role %q", role)
	}

	normalized, err := parseTunMode(mode)
	if err != nil {
		return "", err
	}

	configPath, err := resolveConfigPath(path, trimmedRole)
	if err != nil {
		return "", err
	}
	if strings.ToLower(filepath.Ext(configPath)) != ".toml" {
		return "", fmt.Errorf("config: only toml files are supported for tun mode updates")
	}

	tree, err := loadOrCreateModeToml(configPath, trimmedRole)
	if err != nil {
		return "", err
	}
	tree.SetPath([]string{trimmedRole, "tun_mode"}, normalized)

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

// EnsureTunMode stores tun_mode defaults in the role config when missing.
func EnsureTunMode(path string, role string, mode string) (string, error) {
	trimmedRole := strings.ToLower(strings.TrimSpace(role))
	if trimmedRole != "client" {
		return "", fmt.Errorf("config: unsupported role %q", role)
	}

	normalized, err := parseTunMode(mode)
	if err != nil {
		return "", err
	}

	configPath, err := resolveConfigPath(path, trimmedRole)
	if err != nil {
		return "", err
	}
	if strings.ToLower(filepath.Ext(configPath)) != ".toml" {
		return "", fmt.Errorf("config: only toml files are supported for tun mode settings")
	}

	tree, err := loadOrCreateModeToml(configPath, trimmedRole)
	if err != nil {
		return "", err
	}

	if tree.GetPath([]string{trimmedRole, "tun_mode"}) != nil {
		return configPath, nil
	}
	tree.SetPath([]string{trimmedRole, "tun_mode"}, normalized)

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

func parseTunMode(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "split":
		return "split", nil
	case "full":
		return "full", nil
	default:
		return "", fmt.Errorf("config: invalid tun mode %q (use split or full)", value)
	}
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

// ClearServerCertificateOverrides resets server certificate fields in the specified config file.
func ClearServerCertificateOverrides(path string) (string, error) {
	configPath, err := resolveConfigPath(path, "server")
	if err != nil {
		return "", err
	}
	if strings.ToLower(filepath.Ext(configPath)) != ".toml" {
		return "", fmt.Errorf("config: only toml files are supported for server certificate updates")
	}

	tree, err := loadOrCreateToml(configPath)
	if err != nil {
		return "", err
	}
	tree.SetPath([]string{"server", "cert_store"}, "")
	tree.SetPath([]string{"server", "certificate"}, "")
	tree.SetPath([]string{"server", "key"}, "")

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

func resolveConfigPath(explicit, role string) (string, error) {
	if trimmed := strings.TrimSpace(explicit); trimmed != "" {
		return filepath.Clean(trimmed), nil
	}
	if strings.EqualFold(strings.TrimSpace(role), "client") {
		return filepath.Clean(PendingConfigPath(layout.ClientConfigFileName)), nil
	}
	if strings.EqualFold(strings.TrimSpace(role), "server") {
		return filepath.Clean(PendingConfigPath(layout.ServerConfigFileName)), nil
	}
	return "", fmt.Errorf("config: unsupported role %q", role)
}

func loadOrCreateModeToml(path string, role string) (*toml.Tree, error) {
	if _, err := os.Stat(path); err == nil {
		return loadOrCreateToml(path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	return toml.TreeFromMap(map[string]any{})
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
		return nil, fmt.Errorf("%w: %s: %v", ErrConfigParse, path, err)
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
