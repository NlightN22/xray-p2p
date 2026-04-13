//go:build linux || windows

package server

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

var runRequiredConfigFiles = []string{"inbounds.json", "logs.json", "outbounds.json", "routing.json"}

func HasRunConfigFiles(dir string) (bool, error) {
	return configFilesPresent(dir, runRequiredConfigFiles)
}

func adjustRunPaths(configDir string) (string, string, error) {
	liveConfig := filepath.Clean(config.ConfigPath(layout.ServerConfigFileName))
	pendingConfig := filepath.Clean(config.PendingConfigPath(layout.ServerConfigFileName))
	configFile := liveConfig
	if _, err := os.Stat(liveConfig); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if pendingConfig != "" {
				if _, pendingErr := os.Stat(pendingConfig); pendingErr == nil {
					configFile = pendingConfig
				} else if !errors.Is(pendingErr, os.ErrNotExist) {
					return "", "", pendingErr
				}
			}
		} else {
			return "", "", err
		}
	}

	if ok, err := configFilesPresent(configDir, runRequiredConfigFiles); err != nil {
		return "", "", err
	} else if !ok {
		pendingDir := apply.PendingDir(configDir)
		if ok, err := configFilesPresent(pendingDir, runRequiredConfigFiles); err != nil {
			return "", "", err
		} else if ok {
			configDir = pendingDir
		}
	}

	return configDir, configFile, nil
}

func configFilesPresent(dir string, names []string) (bool, error) {
	for _, name := range names {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return false, nil
			}
			return false, err
		}
	}
	return true, nil
}
