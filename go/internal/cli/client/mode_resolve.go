package clientcmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func resolveClientMode(cfg config.Config) (string, error) {
	path := config.ConfigPath(layout.ClientAppliedStateFileName)
	data, err := os.ReadFile(path)
	if err == nil {
		mode, parseErr := parseModeFromState(data)
		if parseErr != nil {
			return "", fmt.Errorf("parse applied client mode state: %w", parseErr)
		}
		return mode, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read applied client mode state: %w", err)
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

func parseModeFromState(data []byte) (string, error) {
	var state struct {
		Mode       string `json:"mode"`
		TunEnabled bool   `json:"tun_enabled"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return "", err
	}
	mode := strings.ToLower(strings.TrimSpace(state.Mode))
	if mode == "tun" || mode == "proxy" {
		return mode, nil
	}
	if state.TunEnabled {
		return "tun", nil
	}
	return "proxy", nil
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
