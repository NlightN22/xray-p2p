package client

import (
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/xrayconfig"
)

func ensureClientXrayConfig(configFile string) (xrayconfig.ClientXrayConfig, error) {
	return xrayconfig.EnsureClientConfig(configFile, config.ConfigPath(layout.AuditLogFileName))
}

func loadClientTunSettings(configFile string) (bool, string, int, error) {
	cfg, err := config.Load(config.Options{Path: configFile})
	if err != nil {
		return false, "", 0, err
	}
	return cfg.Client.TunEnabled, cfg.Client.TunName, cfg.Client.TunMTU, nil
}
