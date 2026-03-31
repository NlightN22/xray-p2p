package server

import (
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func rebuildServerRouting(installDir string, configDir string) error {
	configPath := filepath.Clean(config.ConfigPath(layout.ServerConfigFileName))
	return rebuildServerRoutingFromPath(configPath, configDir)
}

func rebuildServerRoutingFromPath(configPath string, configDir string) error {
	desired, err := loadServerDesiredConfigFromPath(configPath)
	if err != nil {
		return err
	}
	xrayCfg, err := ensureServerXrayConfig(filepath.Clean(configPath))
	if err != nil {
		return err
	}
	return writeServerRouting(configDir, xrayCfg, desired.Reverse, desired.Redirects)
}
