package client

import (
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/installstate"
)

type clientPaths struct {
	installDir string
	configDir  string
	stateFile  string
}

func resolveClientPaths(installDir, configDir string) (clientPaths, error) {
	dir, err := resolveInstallDir(installDir)
	if err != nil {
		return clientPaths{}, err
	}
	cfgDir, err := resolveConfigDir(dir, configDir)
	if err != nil {
		return clientPaths{}, err
	}
	return clientPaths{
		installDir: dir,
		configDir:  cfgDir,
		stateFile:  filepath.Join(dir, installstate.FileNameForKind(installstate.KindClient)),
	}, nil
}
