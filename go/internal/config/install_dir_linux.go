//go:build linux

package config

import (
	"os"
	"path/filepath"
)

func osPreferredInstallDir() string {
	if os.Geteuid() == 0 {
		return filepath.Join("/opt", "xp2p")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".local", "share", "xp2p")
}
