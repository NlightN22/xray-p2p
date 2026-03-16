package common

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

type InstallRole string

const (
	InstallRoleClient InstallRole = "client"
	InstallRoleServer InstallRole = "server"
)

// InstallPresent checks whether an installation is present for the given role.
func InstallPresent(role InstallRole, installDir, configDirName string) (bool, error) {
	configPath, statePath, err := roleConfigPaths(role)
	if err != nil {
		return false, err
	}
	if found, err := pathExists(configPath); err != nil {
		return false, err
	} else if found {
		return true, nil
	}
	if found, err := pathExists(statePath); err != nil {
		return false, err
	} else if found {
		return true, nil
	}
	configDir, err := resolveConfigDir(role, installDir, configDirName)
	if err != nil {
		return false, err
	}
	inboundsPath := filepath.Join(configDir, "inbounds.json")
	if found, err := pathExists(inboundsPath); err != nil {
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

func resolveConfigDir(role InstallRole, installDir, configDirName string) (string, error) {
	installDir = strings.TrimSpace(installDir)
	if installDir == "" {
		return "", errors.New("xp2p: install directory is required")
	}
	switch role {
	case InstallRoleClient:
		return client.ResolveConfigDir(installDir, configDirName)
	case InstallRoleServer:
		return server.ResolveConfigDir(installDir, configDirName)
	default:
		return "", fmt.Errorf("xp2p: unknown install role %q", role)
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
