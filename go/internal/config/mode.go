package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return "", fmt.Errorf("config: create directory %s: %w", filepath.Dir(configPath), err)
	}
	if err := os.WriteFile(configPath, []byte(tree.String()), 0o644); err != nil {
		return "", fmt.Errorf("config: write %s: %w", configPath, err)
	}
	return configPath, nil
}

func resolveConfigPath(explicit, role string) (string, error) {
	if trimmed := strings.TrimSpace(explicit); trimmed != "" {
		return filepath.Clean(trimmed), nil
	}
	if strings.EqualFold(strings.TrimSpace(role), "client") {
		return filepath.Clean(layout.ClientConfigFileName), nil
	}
	if strings.EqualFold(strings.TrimSpace(role), "server") {
		return filepath.Clean(layout.ServerConfigFileName), nil
	}
	return "", fmt.Errorf("config: unsupported role %q", role)
}

func loadOrCreateToml(path string) (*toml.Tree, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return toml.TreeFromMap(map[string]any{}), nil
		}
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	if strings.TrimSpace(string(data)) == "" {
		return toml.TreeFromMap(map[string]any{}), nil
	}
	tree, err := toml.LoadBytes(data)
	if err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return tree, nil
}
