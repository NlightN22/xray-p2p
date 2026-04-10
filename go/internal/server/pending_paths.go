//go:build linux || windows

package server

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/configio"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func pendingConfigPath() string {
	return filepath.Clean(config.PendingConfigPath(layout.ServerConfigFileName))
}

func pendingConfigDir(liveConfigDir string) string {
	return apply.PendingDir(liveConfigDir)
}

func readConfigWithFallback(pendingPath, livePath string) ([]byte, error) {
	if strings.TrimSpace(pendingPath) != "" {
		if data, err := os.ReadFile(pendingPath); err == nil {
			return data, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("xp2p: read %s: %w", pendingPath, err)
		}
	}
	data, err := os.ReadFile(livePath)
	if err != nil {
		return nil, fmt.Errorf("xp2p: read %s: %w", livePath, err)
	}
	return data, nil
}

func loadServerConfigWithFallback() (config.Config, error) {
	pendingPath := pendingConfigPath()
	if pendingPath != "" {
		if _, err := os.Stat(pendingPath); err == nil {
			return config.Load(config.Options{Path: pendingPath})
		} else if !errors.Is(err, os.ErrNotExist) {
			return config.Config{}, err
		}
	}
	return config.Load(config.Options{Path: config.ConfigPath(layout.ServerConfigFileName)})
}

func resolveServerConfigPath() (string, error) {
	livePath := filepath.Clean(config.ConfigPath(layout.ServerConfigFileName))
	pendingPath := pendingConfigPath()
	if pendingPath != "" {
		if _, err := os.Stat(pendingPath); err == nil {
			return pendingPath, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	return livePath, nil
}

func ensurePendingServerConfigFile(pendingPath, livePath string) error {
	if strings.TrimSpace(pendingPath) == "" || strings.TrimSpace(livePath) == "" {
		return nil
	}
	if _, err := os.Stat(pendingPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	data, err := os.ReadFile(livePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(pendingPath), 0o755); err != nil {
		return err
	}
	return configio.WriteBytes(pendingPath, data, configio.WriteOptions{
		AuditPath:         config.AuditLogPath(),
		IgnoreAuditErrors: true,
	})
}

func writeServerApplyRequest() error {
	req, err := apply.NewRequest(apply.RoleServer)
	if err != nil {
		return err
	}
	return apply.WriteRequest(config.ApplyRequestPath(), req, config.AuditLogPath())
}
