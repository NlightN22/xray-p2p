package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func RoleConfigFileName(role string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "client":
		return layout.ClientConfigFileName, nil
	case "server":
		return layout.ServerConfigFileName, nil
	default:
		return "", fmt.Errorf("config: unsupported role %q", role)
	}
}

func RoleConfigDirName(role string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "client":
		return layout.ClientConfigDir, nil
	case "server":
		return layout.ServerConfigDir, nil
	default:
		return "", fmt.Errorf("config: unsupported role %q", role)
	}
}

func DesiredConfigPathForRole(role string) (string, error) {
	name, err := RoleConfigFileName(role)
	if err != nil {
		return "", err
	}
	return filepath.Clean(ConfigPath(name)), nil
}

func DesiredExtensionsDirForRole(role string) (string, error) {
	configPath, err := DesiredConfigPathForRole(role)
	if err != nil {
		return "", err
	}

	var cfg Config
	if _, statErr := os.Stat(configPath); statErr == nil {
		loaded, err := Load(Options{Path: configPath, AllowInvalid: true})
		if err != nil {
			return "", err
		}
		cfg = loaded
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return "", fmt.Errorf("config: stat %s: %w", configPath, statErr)
	} else {
		cfg = Config{}
		normalize(&cfg)
	}

	name := ""
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "client":
		name = strings.TrimSpace(cfg.Client.ConfigDir)
	case "server":
		name = strings.TrimSpace(cfg.Server.ConfigDir)
	default:
		return "", fmt.Errorf("config: unsupported role %q", role)
	}
	if name == "" {
		name, err = RoleConfigDirName(role)
		if err != nil {
			return "", err
		}
	}
	return filepath.Clean(ConfigPath(name)), nil
}

func LiveRoleDir(role string) (string, error) {
	name, err := RoleConfigDirName(role)
	if err != nil {
		return "", err
	}
	return filepath.Join(LiveRoot(), name), nil
}

func LkgRoleDir(role string) (string, error) {
	name, err := RoleConfigDirName(role)
	if err != nil {
		return "", err
	}
	return filepath.Join(LkgRoot(), name), nil
}

func LiveXrayPath(role string) (string, error) {
	dir, err := LiveRoleDir(role)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, layout.XrayConfigFileName), nil
}

func LkgXrayPath(role string) (string, error) {
	dir, err := LkgRoleDir(role)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, layout.XrayConfigFileName), nil
}

func LiveRuntimeMetaPath(role string) (string, error) {
	dir, err := LiveRoleDir(role)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, layout.RuntimeMetaFileName), nil
}

func LkgRuntimeMetaPath(role string) (string, error) {
	dir, err := LkgRoleDir(role)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, layout.RuntimeMetaFileName), nil
}
