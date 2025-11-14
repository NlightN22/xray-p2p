//go:build linux

package xray

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

// ResolveBinaryPath returns the location of the xray binary on Linux hosts.
func ResolveBinaryPath() (string, error) {
	if env := strings.TrimSpace(os.Getenv("XP2P_XRAY_BIN")); env != "" {
		return env, nil
	}

	systemPath := filepath.Join(layout.UnixConfigRoot, layout.BinDirName, "xray")
	if stat, err := os.Stat(systemPath); err == nil {
		if stat.IsDir() {
			return "", fmt.Errorf("xp2p: %s is a directory, expected xray binary", systemPath)
		}
		return systemPath, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("xp2p: inspect xray binary at %s: %w", systemPath, err)
	}

	path, err := exec.LookPath("xray")
	if err != nil {
		return "", fmt.Errorf("xp2p: xray binary not found in PATH or %s (set XP2P_XRAY_BIN): %w", systemPath, err)
	}
	return path, nil
}
