package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

const envLogRoot = "XP2P_LOG_ROOT"

// ConfigRoot returns the default configuration root for the current platform.
func ConfigRoot() string {
	if override := strings.TrimSpace(os.Getenv("XP2P_CONFIG_ROOT")); override != "" {
		return override
	}
	return defaultConfigRoot()
}

// ConfigPath returns the full path inside the configuration root.
func ConfigPath(name string) string {
	root := filepath.Clean(ConfigRoot())
	if root == "." || root == "" {
		return filepath.Clean(name)
	}
	return filepath.Join(root, filepath.Clean(name))
}

// StateRoot returns the configuration state root directory.
func StateRoot() string {
	return filepath.Join(filepath.Clean(ConfigRoot()), layout.StateDirName)
}

// PendingRoot returns the configuration pending root directory.
func PendingRoot() string {
	return filepath.Join(StateRoot(), layout.PendingDirName)
}

// LiveRoot returns the configuration live root directory.
func LiveRoot() string {
	return filepath.Join(StateRoot(), layout.LiveDirName)
}

// LkgRoot returns the configuration last known good root directory.
func LkgRoot() string {
	return filepath.Join(StateRoot(), layout.LkgDirName)
}

// PendingConfigPath returns the full path inside the pending root.
func PendingConfigPath(name string) string {
	root := filepath.Clean(PendingRoot())
	if root == "." || root == "" {
		return filepath.Clean(name)
	}
	return filepath.Join(root, filepath.Clean(name))
}

// LiveConfigPath returns the full path inside the live root.
func LiveConfigPath(name string) string {
	root := filepath.Clean(LiveRoot())
	if root == "." || root == "" {
		return filepath.Clean(name)
	}
	return filepath.Join(root, filepath.Clean(name))
}

// LkgConfigPath returns the full path inside the last known good root.
func LkgConfigPath(name string) string {
	root := filepath.Clean(LkgRoot())
	if root == "." || root == "" {
		return filepath.Clean(name)
	}
	return filepath.Join(root, filepath.Clean(name))
}

// ApplyRequestPath returns the full path to apply.request.
func ApplyRequestPath() string {
	return filepath.Join(StateRoot(), layout.ApplyRequestFileName)
}

// LogRoot returns the default log root for the current platform.
func LogRoot() string {
	if override := strings.TrimSpace(os.Getenv(envLogRoot)); override != "" {
		return filepath.Clean(override)
	}
	return defaultLogRoot()
}

// LogPath returns the full path inside the log root.
func LogPath(name string) string {
	root := filepath.Clean(LogRoot())
	if root == "." || root == "" {
		return filepath.Clean(name)
	}
	return filepath.Join(root, filepath.Clean(name))
}

// AuditLogPath returns the full audit log path.
func AuditLogPath() string {
	return LogPath(layout.AuditLogFileName)
}
