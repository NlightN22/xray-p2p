package common

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

type InstallRole string

const (
	InstallRoleClient InstallRole = "client"
	InstallRoleServer InstallRole = "server"
)

// InstallPresent checks whether an installation is present for the given role.
func InstallPresent(role InstallRole, installDir, configDirName string) (bool, error) {
	desiredPath, statePath, err := roleConfigPaths(role)
	if err != nil {
		return false, err
	}
	if found, err := pathExists(desiredPath); err != nil {
		return false, err
	} else if found {
		return true, nil
	}
	if found, err := pathExists(statePath); err != nil {
		return false, err
	} else if found {
		return true, nil
	}
	liveXray, err := config.LiveXrayPath(string(role))
	if err != nil {
		return false, err
	}
	if found, err := pathExists(liveXray); err != nil {
		return false, err
	} else if found {
		return true, nil
	}
	return false, nil
}

func roleConfigPaths(role InstallRole) (string, string, error) {
	switch role {
	case InstallRoleClient:
		return filepath.Clean(config.ConfigPath(layout.ClientConfigFileName)),
			filepath.Clean(config.ConfigPath(layout.ClientAppliedStateFileName)),
			nil
	case InstallRoleServer:
		return filepath.Clean(config.ConfigPath(layout.ServerConfigFileName)),
			filepath.Clean(config.ConfigPath(layout.ServerAppliedStateFileName)),
			nil
	default:
		return "", "", fmt.Errorf("xp2p: unknown install role %q", role)
	}
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}
