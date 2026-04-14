//go:build linux || windows

package client

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/configio"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func resolvePendingClientPaths(installDir, configDir string) (clientPaths, error) {
	dir, err := resolveInstallDir(installDir)
	if err != nil {
		return clientPaths{}, err
	}
	desiredConfigDir, err := ResolveConfigDir(dir, configDir)
	if err != nil {
		return clientPaths{}, err
	}
	pendingDir, err := config.PendingConfigDir(desiredConfigDir)
	if err != nil {
		return clientPaths{}, err
	}
	liveDir, err := config.LiveConfigDir(desiredConfigDir)
	if err != nil {
		return clientPaths{}, err
	}
	pendingConfigFile := filepath.Clean(config.PendingConfigPath(layout.ClientConfigFileName))
	liveConfigFile := filepath.Clean(config.LiveConfigPath(layout.ClientConfigFileName))
	if err := ensurePendingConfigFile(pendingConfigFile, liveConfigFile); err != nil {
		return clientPaths{}, err
	}
	if err := ensurePendingConfigSnapshot(pendingDir, liveDir); err != nil {
		return clientPaths{}, err
	}
	return clientPaths{
		installDir: dir,
		configDir:  pendingDir,
		configFile: pendingConfigFile,
		stateFile:  filepath.Clean(config.ConfigPath(layout.ClientAppliedStateFileName)),
		fullState:  filepath.Clean(config.ConfigPath(layout.ClientFullTunnelStateFileName)),
	}, nil
}

func ensurePendingConfigFile(pendingPath, livePath string) error {
	if pendingPath == "" || livePath == "" {
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
