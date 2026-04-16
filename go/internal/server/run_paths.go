//go:build linux || windows

package server

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

var runRequiredConfigFiles = []string{layout.XrayConfigFileName, layout.RuntimeMetaFileName}

func HasRunConfigFiles(dir string) (bool, error) {
	return configFilesPresent(dir, runRequiredConfigFiles)
}

func adjustRunPaths(configDir string) (string, string, error) {
	if ok, err := configFilesPresent(configDir, runRequiredConfigFiles); err != nil {
		return "", "", err
	} else if !ok {
		return "", "", os.ErrNotExist
	}

	return configDir, filepath.Join(configDir, layout.XrayConfigFileName), nil
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
