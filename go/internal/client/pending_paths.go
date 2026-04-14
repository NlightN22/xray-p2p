//go:build linux || windows

package client

import (
	"errors"
	"os"
	"path/filepath"

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
	pendingConfigFile := filepath.Clean(config.PendingConfigPath(layout.ClientConfigFileName))
	liveConfigFile := filepath.Clean(config.LiveConfigPath(layout.ClientConfigFileName))
	if err := ensurePendingConfigFile(pendingConfigFile, liveConfigFile); err != nil {
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
