package server

import (
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/xrayconfig"
)

func ensureServerXrayConfig(configFile string) (xrayconfig.ServerXrayConfig, error) {
	return xrayconfig.EnsureServerConfig(configFile, config.ConfigPath(layout.AuditLogFileName))
}

func loadServerTunSettings(configFile string) (bool, string, int, error) {
	cfg, err := config.Load(config.Options{Path: configFile})
	if err != nil {
		return false, "", 0, err
	}
	return cfg.Server.TunEnabled, cfg.Server.TunName, cfg.Server.TunMTU, nil
}
