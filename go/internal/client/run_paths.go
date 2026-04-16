//go:build linux || windows

package client

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

var runRequiredConfigFiles = []string{layout.XrayConfigFileName, layout.RuntimeMetaFileName}

func adjustRunPaths(paths clientPaths) (clientPaths, error) {
	if ok, err := configFilesPresent(paths.configDir, runRequiredConfigFiles); err != nil {
		return paths, err
	} else if !ok {
		return paths, os.ErrNotExist
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
