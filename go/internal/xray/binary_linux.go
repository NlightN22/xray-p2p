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

// ResolveBinaryPathForInstallDir returns the location of the xray binary for the supplied install dir.
func ResolveBinaryPathForInstallDir(installDir string) (string, error) {
	if env := strings.TrimSpace(os.Getenv("XP2P_XRAY_BIN")); env != "" {
		return env, nil
	}

	dir := strings.TrimSpace(installDir)
	if dir == "" {
		dir = layout.UnixConfigRoot
	}
	systemPath := filepath.Join(dir, layout.BinDirName, "xray")
	if stat, err := os.Stat(systemPath); err == nil {
		if stat.IsDir() {
			return "", fmt.Errorf("%s is a directory, expected xray binary", systemPath)
		}
		return systemPath, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect xray binary at %s: %w", systemPath, err)
	}

	path, err := exec.LookPath("xray")
	if err != nil {
		return "", fmt.Errorf("xray binary not found in PATH or %s (set XP2P_XRAY_BIN): %w", systemPath, err)
	}
	return path, nil
}

// ResolveBinaryPath returns the location of the xray binary on Linux hosts.
func ResolveBinaryPath() (string, error) {
	return ResolveBinaryPathForInstallDir(layout.UnixConfigRoot)
}
