//go:build linux || windows

package server

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func pendingConfigPath() string {
	return filepath.Clean(config.ConfigPath(layout.ServerConfigFileName))
}

func pendingConfigDir(configDir string) (string, error) {
	if strings.TrimSpace(configDir) == "" {
		return "", fmt.Errorf("config dir is empty")
	}
	return filepath.Clean(configDir), nil
}

func loadServerConfigWithFallback() (config.Config, error) {
	return config.Load(config.Options{Path: pendingConfigPath()})
}

func resolveServerConfigPath() (string, error) {
	return pendingConfigPath(), nil
}

func writeServerApplyRequest() error {
	req, err := apply.NewRequest(apply.RoleServer)
	if err != nil {
		return err
	}
	return apply.WriteRequest(config.ApplyRequestPath(), req, config.AuditLogPath())
}
