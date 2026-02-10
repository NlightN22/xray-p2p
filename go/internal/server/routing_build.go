package server

import (
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func rebuildServerRouting(installDir string, configDir string) error {
	desired, err := loadServerDesiredConfig(installDir)
	if err != nil {
		return err
	}
	xrayCfg, err := ensureServerXrayConfig(filepath.Clean(config.ConfigPath(layout.ServerConfigFileName)))
	if err != nil {
		return err
	}
	return writeServerRouting(configDir, xrayCfg, desired.Reverse, desired.Redirects)
}
