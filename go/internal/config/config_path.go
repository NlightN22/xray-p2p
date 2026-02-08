package config

import (
	"os"
	"path/filepath"
	"strings"
)

// ConfigRoot returns the default configuration root for the current platform.
func ConfigRoot() string {
	if override := strings.TrimSpace(os.Getenv("XP2P_CONFIG_ROOT")); override != "" {
		return override
	}
	return defaultInstallDir()
}

// ConfigPath returns the full path inside the configuration root.
func ConfigPath(name string) string {
	root := filepath.Clean(ConfigRoot())
	if root == "." || root == "" {
		return filepath.Clean(name)
	}
	return filepath.Join(root, filepath.Clean(name))
}
