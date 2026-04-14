package client

import (
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

type clientPaths struct {
	installDir string
	configDir  string
	configFile string
	stateFile  string
	fullState  string
}

func resolveClientPaths(installDir, configDir string) (clientPaths, error) {
	dir, err := resolveInstallDir(installDir)
	if err != nil {
		return clientPaths{}, err
	}
	desiredConfigDir, err := ResolveConfigDir(dir, configDir)
	if err != nil {
		return clientPaths{}, err
	}
	liveConfigDir, err := config.LiveConfigDir(desiredConfigDir)
	if err != nil {
		return clientPaths{}, err
	}
	return clientPaths{
		installDir: dir,
		configDir:  liveConfigDir,
		configFile: filepath.Clean(config.LiveConfigPath(layout.ClientConfigFileName)),
		stateFile:  filepath.Clean(config.ConfigPath(layout.ClientAppliedStateFileName)),
		fullState:  filepath.Clean(config.ConfigPath(layout.ClientFullTunnelStateFileName)),
	}, nil
}
