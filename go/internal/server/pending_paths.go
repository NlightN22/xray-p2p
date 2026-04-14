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

func pendingConfigDir(configDir string) (string, error) {
	pendingDir, err := config.PendingConfigDir(configDir)
	if err != nil {
		return "", err
	}
	liveDir, err := config.LiveConfigDir(configDir)
	if err != nil {
		return "", err
	}
	if err := ensurePendingConfigSnapshot(pendingDir, liveDir); err != nil {
		return "", err
	}
	return pendingDir, nil
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
	return config.Load(config.Options{Path: config.LiveConfigPath(layout.ServerConfigFileName)})
}

func resolveServerConfigPath() (string, error) {
	livePath := filepath.Clean(config.LiveConfigPath(layout.ServerConfigFileName))
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

func ensurePendingConfigSnapshot(pendingDir, liveDir string) error {
	if strings.TrimSpace(pendingDir) == "" || strings.TrimSpace(liveDir) == "" {
		return nil
	}
	info, err := os.Stat(liveDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("xp2p: %s is not a directory", liveDir)
	}
	return filepath.WalkDir(liveDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(liveDir, path)
		if err != nil {
			return err
		}
		pendingPath := filepath.Join(pendingDir, rel)
		if _, err := os.Stat(pendingPath); err == nil {
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(pendingPath), 0o755); err != nil {
			return err
		}
		return configio.WriteBytes(pendingPath, data, configio.WriteOptions{
			AuditPath:         config.AuditLogPath(),
			IgnoreAuditErrors: true,
		})
	})
}

func writeServerApplyRequest() error {
	req, err := apply.NewRequest(apply.RoleServer)
	if err != nil {
		return err
	}
	return apply.WriteRequest(config.ApplyRequestPath(), req, config.AuditLogPath())
}
