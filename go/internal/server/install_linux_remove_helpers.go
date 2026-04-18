//go:build linux

package server

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/installstate"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func removeInstallDirIfUnused(installDir string) error {
	clientStatePath := filepath.Join(installDir, installstate.FileNameForKind(installstate.KindClient))
	if installedRole(clientStatePath, installstate.KindClient) {
		return nil
	}
	serverStatePath := filepath.Join(installDir, installstate.FileNameForKind(installstate.KindServer))
	if installedRole(serverStatePath, installstate.KindServer) {
		return nil
	}
	legacyStatePath := filepath.Join(installDir, layout.StateFileName)
	if legacyHasRoles(legacyStatePath) {
		return nil
	}
	if dirHasFiles(filepath.Join(installDir, layout.BinDirName)) {
		return nil
	}
	if err := os.RemoveAll(installDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove install dir: %w", err)
	}
	return nil
}

func dirHasFiles(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	return len(entries) > 0
}

func removeNetworkdConfig(tunName string) error {
	name := strings.TrimSpace(tunName)
	if name == "" {
		return nil
	}
	path := filepath.Join("/etc/systemd/network", fmt.Sprintf("90-%s.network", name))
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove networkd config: %w", err)
	}
	return nil
}

func installedRole(path string, kind installstate.Kind) bool {
	if _, err := installstate.Read(path, kind); err == nil {
		return true
	} else if errors.Is(err, os.ErrNotExist) || errors.Is(err, installstate.ErrRoleNotInstalled) {
		return false
	}
	return true
}

func legacyHasRoles(path string) bool {
	roles, err := installstate.Roles(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false
		}
		return true
	}
	return len(roles) > 0
}
