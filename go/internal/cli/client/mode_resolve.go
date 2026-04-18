package clientcmd

import (
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func resolveClientMode(cfg config.Config) (string, error) {
	path := config.ConfigPath(layout.ClientAppliedStateFileName)
	data, err := os.ReadFile(path)
	if err == nil {
		if mode := parseModeFromState(data); mode != "" {
			return mode, nil
		}
	}
	if cfg.Client.TunEnabled {
		return "tun", nil
	}
	return "proxy", nil
}

func resolveClientTunMode(configPath string, cfg config.Config) (string, error) {
	trimmed := strings.TrimSpace(configPath)
	if trimmed == "" {
		if cfg.Client.TunMode != "" {
			return cfg.Client.TunMode, nil
		}
		trimmed = resolveConfigPath(layout.ClientConfigFileName)
	}
	loaded, err := config.Load(config.Options{
		Path:         trimmed,
		AllowInvalid: true,
	})
	if err != nil {
		return "", err
	}
	return loaded.Client.TunMode, nil
}

func parseModeFromState(data []byte) string {
	var state struct {
		Mode       string `json:"mode"`
		TunEnabled bool   `json:"tun_enabled"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return ""
	}
	mode := strings.ToLower(strings.TrimSpace(state.Mode))
	if mode == "tun" || mode == "proxy" {
		return mode
	}
	if state.TunEnabled {
		return "tun"
	}
	return "proxy"
}

func parseMode(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "tun":
		return true, nil
	case "proxy":
		return false, nil
	default:
		return false, errors.New("use tun or proxy")
	}
}

func loadModeConfig(configPath string, fallback config.Config) (config.Config, error) {
	trimmed := strings.TrimSpace(configPath)
	if trimmed == "" {
		trimmed = resolveConfigPath(layout.ClientConfigFileName)
	}
	loaded, err := config.Load(config.Options{
		Path:         trimmed,
		AllowInvalid: true,
	})
	if err != nil {
		return fallback, err
	}
	return loaded, nil
}

func resolveConfigPath(name string) string {
	pending := config.PendingConfigPath(name)
	if _, err := os.Stat(pending); err == nil {
		return pending
	}
	live := config.LiveConfigPath(name)
	if _, err := os.Stat(live); err == nil {
		return live
	}
	desired := config.ConfigPath(name)
	if _, err := os.Stat(desired); err == nil {
		return desired
	}
	return pending
}
