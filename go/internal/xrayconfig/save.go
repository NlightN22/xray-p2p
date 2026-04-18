package xrayconfig

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/pelletier/go-toml"

	"github.com/NlightN22/xray-p2p/go/internal/configio"
)

func EnsureClientConfig(path string, auditPath string) (ClientXrayConfig, error) {
	cfg := DefaultClientConfig()
	if strings.TrimSpace(path) == "" {
		return cfg, nil
	}
	tree, err := loadOrCreateToml(path)
	if err != nil {
		return ClientXrayConfig{}, err
	}
	raw := tree.GetPath([]string{"client", "xray"})
	loaded, ok, err := decodeClientConfig(raw)
	if err != nil {
		return ClientXrayConfig{}, err
	}
	merged := mergeClientConfig(loaded, cfg)
	if err := validateClientConfig(merged); err != nil {
		return ClientXrayConfig{}, err
	}
	if ok && reflect.DeepEqual(loaded, merged) {
		return merged, nil
	}
	if err := writeConfigTree(path, tree, "client", merged, auditPath); err != nil {
		return ClientXrayConfig{}, err
	}
	return merged, nil
}

func EnsureServerConfig(path string, auditPath string) (ServerXrayConfig, error) {
	cfg := DefaultServerConfig()
	if strings.TrimSpace(path) == "" {
		return cfg, nil
	}
	tree, err := loadOrCreateToml(path)
	if err != nil {
		return ServerXrayConfig{}, err
	}
	raw := tree.GetPath([]string{"server", "xray"})
	loaded, ok, err := decodeServerConfig(raw)
	if err != nil {
		return ServerXrayConfig{}, err
	}
	merged := mergeServerConfig(loaded, cfg)
	if err := validateServerConfig(merged); err != nil {
		return ServerXrayConfig{}, err
	}
	if ok && reflect.DeepEqual(loaded, merged) {
		return merged, nil
	}
	if err := writeConfigTree(path, tree, "server", merged, auditPath); err != nil {
		return ServerXrayConfig{}, err
	}
	return merged, nil
}

func SaveClientConfig(path string, auditPath string, cfg ClientXrayConfig) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := validateClientConfig(cfg); err != nil {
		return err
	}
	tree, err := loadOrCreateToml(path)
	if err != nil {
		return err
	}
	return writeConfigTree(path, tree, "client", cfg, auditPath)
}

func SaveServerConfig(path string, auditPath string, cfg ServerXrayConfig) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := validateServerConfig(cfg); err != nil {
		return err
	}
	tree, err := loadOrCreateToml(path)
	if err != nil {
		return err
	}
	return writeConfigTree(path, tree, "server", cfg, auditPath)
}

func writeConfigTree(path string, tree *toml.Tree, role string, cfg any, auditPath string) error {
	section := strings.ToLower(strings.TrimSpace(role))
	if section == "" {
		return errors.New("xrayconfig: empty role section")
	}
	cfgMap, err := toMap(cfg)
	if err != nil {
		return err
	}
	tree.SetPath([]string{section, "xray"}, cfgMap)
	data, err := toml.Marshal(tree.ToMap())
	if err != nil {
		return fmt.Errorf("xrayconfig: encode toml %s: %w", path, err)
	}
	return configio.WriteBytes(path, data, configio.WriteOptions{
		AuditPath:         auditPath,
		KeepLastKnownGood: false,
	})
}
