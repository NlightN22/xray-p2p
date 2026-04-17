package server

import (
	"errors"
	"fmt"
	"os"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/xrayconfig"
)

func ensureServerXrayConfig(configFile string) (xrayconfig.ServerXrayConfig, error) {
	return xrayconfig.EnsureServerConfig(configFile, config.AuditLogPath())
}

func EnsureServerXrayConfig(configFile string) (xrayconfig.ServerXrayConfig, error) {
	return ensureServerXrayConfig(configFile)
}

func loadServerXrayConfig(configFile string) (xrayconfig.ServerXrayConfig, error) {
	cfg, err := xrayconfig.LoadServerConfig(configFile)
	if err == nil {
		return cfg, nil
	}
	if errors.Is(err, xrayconfig.ErrConfigMissing) || errors.Is(err, xrayconfig.ErrConfigEmpty) {
		return cfg, fmt.Errorf("server config is missing at %s (run install or deploy first)", configFile)
	}
	return cfg, err
}

func ensureServerXrayConfigForce(configFile string, force bool) (xrayconfig.ServerXrayConfig, error) {
	cfg, err := ensureServerXrayConfig(configFile)
	if err == nil || !force || !errors.Is(err, xrayconfig.ErrConfigParse) {
		return cfg, err
	}
	if removeErr := os.Remove(configFile); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		return cfg, removeErr
	}
	return ensureServerXrayConfig(configFile)
}

func loadServerTunSettings(configFile string) (bool, string, int, error) {
	cfg, err := config.Load(config.Options{Path: configFile})
	if err != nil {
		return false, "", 0, err
	}
	return cfg.Server.TunEnabled, cfg.Server.TunName, cfg.Server.TunMTU, nil
}
