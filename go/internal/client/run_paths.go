//go:build linux || windows

package client

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

var runRequiredConfigFiles = []string{"inbounds.json", "logs.json", "outbounds.json", "routing.json"}

func adjustRunPaths(paths clientPaths) (clientPaths, error) {
	liveConfig := filepath.Clean(config.ConfigPath(layout.ClientConfigFileName))
	pendingConfig := filepath.Clean(config.PendingConfigPath(layout.ClientConfigFileName))
	if paths.configFile == liveConfig {
		if _, err := os.Stat(liveConfig); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				if pendingConfig != "" {
					if _, pendingErr := os.Stat(pendingConfig); pendingErr == nil {
						paths.configFile = pendingConfig
					} else if !errors.Is(pendingErr, os.ErrNotExist) {
						return paths, pendingErr
					}
				}
			} else {
				return paths, err
			}
		}
	}

	if ok, err := configFilesPresent(paths.configDir, runRequiredConfigFiles); err != nil {
		return paths, err
	} else if !ok {
		pendingDir := apply.PendingDir(paths.configDir)
		if ok, err := configFilesPresent(pendingDir, runRequiredConfigFiles); err != nil {
			return paths, err
		} else if ok {
			paths.configDir = pendingDir
		}
	}

	return paths, nil
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
