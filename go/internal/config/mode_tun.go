package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/configio"
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
