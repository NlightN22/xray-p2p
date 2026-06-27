package xrayconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/pelletier/go-toml"
)

func LoadClientConfig(path string) (ClientXrayConfig, error) {
	cfg := DefaultClientConfig()
	if strings.TrimSpace(path) == "" {
		return ClientXrayConfig{}, errors.New("xrayconfig: config path is empty")
	}
	tree, err := loadExistingToml(path)
	if err != nil {
		return ClientXrayConfig{}, err
	}
	raw := tree.GetPath([]string{"client", "xray"})
	loaded, ok, err := decodeClientConfig(raw)
	if err != nil {
		return ClientXrayConfig{}, err
	}
	if !ok {
		return ClientXrayConfig{}, fmt.Errorf("xrayconfig: client xray config not found in %s", path)
	}
	merged := mergeClientConfig(loaded, cfg)
	if err := validateClientConfig(merged); err != nil {
		return ClientXrayConfig{}, err
	}
	return merged, nil
}

// LoadClientConfigWithDefaults returns the merged xray config; missing [client.xray] falls back to defaults.
func LoadClientConfigWithDefaults(path string) (ClientXrayConfig, error) {
	defaults := DefaultClientConfig()
	if strings.TrimSpace(path) == "" {
		return ClientXrayConfig{}, errors.New("xrayconfig: config path is empty")
	}
	tree, err := loadExistingToml(path)
	if err != nil {
		return ClientXrayConfig{}, err
	}
	raw := tree.GetPath([]string{"client", "xray"})
	loaded, ok, err := decodeClientConfig(raw)
	if err != nil {
		return ClientXrayConfig{}, err
	}
	if !ok {
		return defaults, nil
	}
	merged := mergeClientConfig(loaded, defaults)
	if err := validateClientConfig(merged); err != nil {
		return ClientXrayConfig{}, err
	}
	return merged, nil
}

func LoadServerConfig(path string) (ServerXrayConfig, error) {
	cfg := DefaultServerConfig()
	if strings.TrimSpace(path) == "" {
		return ServerXrayConfig{}, errors.New("xrayconfig: config path is empty")
	}
	tree, err := loadExistingToml(path)
	if err != nil {
		return ServerXrayConfig{}, err
	}
	raw := tree.GetPath([]string{"server", "xray"})
	loaded, ok, err := decodeServerConfig(raw)
	if err != nil {
		return ServerXrayConfig{}, err
	}
	if !ok {
		return ServerXrayConfig{}, fmt.Errorf("xrayconfig: server xray config not found in %s", path)
	}
	merged := mergeServerConfig(loaded, cfg)
	if err := validateServerConfig(merged); err != nil {
		return ServerXrayConfig{}, err
	}
	return merged, nil
}

// LoadServerConfigWithDefaults returns the merged xray config; missing [server.xray] falls back to defaults.
func LoadServerConfigWithDefaults(path string) (ServerXrayConfig, error) {
	defaults := DefaultServerConfig()
	if strings.TrimSpace(path) == "" {
		return ServerXrayConfig{}, errors.New("xrayconfig: config path is empty")
	}
	tree, err := loadExistingToml(path)
	if err != nil {
		return ServerXrayConfig{}, err
	}
	raw := tree.GetPath([]string{"server", "xray"})
	loaded, ok, err := decodeServerConfig(raw)
	if err != nil {
		return ServerXrayConfig{}, err
	}
	if !ok {
		return defaults, nil
	}
	merged := mergeServerConfig(loaded, defaults)
	if err := validateServerConfig(merged); err != nil {
		return ServerXrayConfig{}, err
	}
	return merged, nil
}

func decodeClientConfig(raw any) (ClientXrayConfig, bool, error) {
	if raw == nil {
		return ClientXrayConfig{}, false, nil
	}
	if tree, ok := raw.(*toml.Tree); ok {
		raw = tree.ToMap()
	}
	buf, err := json.Marshal(raw)
	if err != nil {
		return ClientXrayConfig{}, false, fmt.Errorf("xrayconfig: encode client xray config: %w", err)
	}
	var cfg ClientXrayConfig
	if err := json.Unmarshal(buf, &cfg); err != nil {
		return ClientXrayConfig{}, false, fmt.Errorf("xrayconfig: decode client xray config: %w", err)
	}
	return cfg, true, nil
}

func decodeServerConfig(raw any) (ServerXrayConfig, bool, error) {
	if raw == nil {
		return ServerXrayConfig{}, false, nil
	}
	if tree, ok := raw.(*toml.Tree); ok {
		raw = tree.ToMap()
	}
	buf, err := json.Marshal(raw)
	if err != nil {
		return ServerXrayConfig{}, false, fmt.Errorf("xrayconfig: encode server xray config: %w", err)
	}
	var cfg ServerXrayConfig
	if err := json.Unmarshal(buf, &cfg); err != nil {
		return ServerXrayConfig{}, false, fmt.Errorf("xrayconfig: decode server xray config: %w", err)
	}
	return cfg, true, nil
}
