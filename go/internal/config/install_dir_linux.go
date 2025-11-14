//go:build linux

package config

import (
	"os"
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func osPreferredInstallDir() string {
	if os.Geteuid() == 0 {
		return layout.UnixConfigRoot
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(os.TempDir(), "xp2p")
	}
	return filepath.Join(home, ".config", "xp2p")
}

func detectSystemInstallDir() string {
	if looksLikeInstallRoot(layout.UnixConfigRoot) {
		return layout.UnixConfigRoot
	}
	return ""
}
