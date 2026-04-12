//go:build linux || windows

package server

import (
	"path/filepath"
	"strings"
)

func applyServerDesiredConfig(installDir, configDir string, desired desiredServerConfig, previousReverse serverReverseState, opts ModeOptions, applyRoutes bool) error {
	previousReverse = normalizeReverse(previousReverse)

	configPath, err := resolveServerConfigPath()
	if err != nil {
		return err
	}
	xrayCfg, err := loadServerXrayConfig(configPath)
	if err != nil {
		return err
	}
	cfg, err := loadServerConfigWithFallback()
	if err != nil {
		return err
	}
	certPath := filepath.Join(configDir, "cert.pem")
	keyPath := filepath.Join(configDir, "key.pem")
	if strings.TrimSpace(cfg.Server.CertificateFile) != "" {
		certPath = cfg.Server.CertificateFile
	}
	if strings.TrimSpace(cfg.Server.KeyFile) != "" {
		keyPath = cfg.Server.KeyFile
	}

	if err := writeServerInboundsConfig(configDir, xrayCfg, opts.TunEnabled, opts.TunName, opts.TunMTU, parsePortOrDefault(cfg.Server.TrojanPort, DefaultTrojanPort), certPath, keyPath, xrayCfg.Inbounds.Trojan.AllowInsecure, desired.Forwards); err != nil {
		return err
	}
	if err := writeServerLogs(configDir, xrayCfg.Logs); err != nil {
		return err
	}
	if err := writeServerOutbounds(configDir, xrayCfg.DirectOutbound); err != nil {
		return err
	}
	if err := writeServerRouting(configDir, xrayCfg, desired.Reverse, desired.Redirects); err != nil {
		return err
	}

	if !applyRoutes {
		return nil
	}
	if opts.TunEnabled {
		return applyRedirectRoutes(opts.TunName, opts.TunAddr, desired.Redirects)
	}
	return removeRedirectRoutes(opts.TunName, opts.TunAddr, desired.Redirects)
}
