//go:build linux || windows

package client

import (
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func resolvePendingClientPaths(installDir, configDir string) (clientPaths, error) {
	dir, err := resolveInstallDir(installDir)
	if err != nil {
		return clientPaths{}, err
	}
	liveConfigDir, err := ResolveConfigDir(dir, configDir)
	if err != nil {
		return clientPaths{}, err
	}
	pendingDir := apply.PendingDir(liveConfigDir)
	return clientPaths{
		installDir: dir,
		configDir:  pendingDir,
		configFile: filepath.Clean(config.PendingConfigPath(layout.ClientConfigFileName)),
		stateFile:  filepath.Clean(config.ConfigPath(layout.ClientAppliedStateFileName)),
		fullState:  filepath.Clean(config.ConfigPath(layout.ClientFullTunnelStateFileName)),
	}, nil
}
