package client

import (
	"errors"
	"os"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/xrayconfig"
)

func ensureClientXrayConfig(configFile string) (xrayconfig.ClientXrayConfig, error) {
	return xrayconfig.EnsureClientConfig(configFile, config.AuditLogPath())
}

func ensureClientXrayConfigForce(configFile string, force bool) (xrayconfig.ClientXrayConfig, error) {
	cfg, err := ensureClientXrayConfig(configFile)
	if err == nil || !force || !errors.Is(err, xrayconfig.ErrConfigParse) {
		return cfg, err
	}
	if removeErr := os.Remove(configFile); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		return cfg, removeErr
	}
	return ensureClientXrayConfig(configFile)
}

func loadClientTunSettings(configFile string) (bool, string, int, error) {
	cfg, err := config.Load(config.Options{Path: configFile})
	if err != nil {
		return false, "", 0, err
	}
	return cfg.Client.TunEnabled, cfg.Client.TunName, cfg.Client.TunMTU, nil
}
