//go:build linux || windows

package server

import (
	"os"
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func applyPendingIfRequested(role string, configDir string) (*apply.Rollback, bool, error) {
	reqPath := config.ApplyRequestPath()
	logging.Debug("apply request check",
		"role", role,
		"apply_request", reqPath,
		"apply_request_exists", fileExists(reqPath),
		"apply_root", config.ApplyRoot(),
		"apply_root_exists", dirExists(config.ApplyRoot()),
		"pending_root", config.PendingRoot(),
		"pending_root_exists", dirExists(config.PendingRoot()),
		"pending_config", config.PendingConfigPath(layout.ServerConfigFileName),
		"pending_config_exists", fileExists(config.PendingConfigPath(layout.ServerConfigFileName)),
		"pending_dir", apply.PendingDir(configDir),
		"pending_dir_exists", dirExists(apply.PendingDir(configDir)),
		"live_config", config.ConfigPath(layout.ServerConfigFileName),
		"live_config_exists", fileExists(config.ConfigPath(layout.ServerConfigFileName)),
		"live_config_dir", configDir,
		"live_config_dir_exists", dirExists(configDir),
	)
	req, exists, err := apply.ReadRequest(reqPath)
	if err != nil {
		return nil, false, err
	}
	if !exists {
		return nil, false, nil
	}
	if !req.MatchesRole(role) {
		return nil, false, nil
	}
	pendingSet := apply.PendingSet{
		LiveConfigFile:    filepath.Clean(config.ConfigPath(layout.ServerConfigFileName)),
		PendingConfigFile: filepath.Clean(config.PendingConfigPath(layout.ServerConfigFileName)),
		LiveConfigDir:     configDir,
		PendingConfigDir:  apply.PendingDir(configDir),
		AuditPath:         config.AuditLogPath(),
	}
	rollback, applied, err := apply.ApplyPending(pendingSet)
	if err != nil {
		return nil, false, err
	}
	if applied {
		if err := apply.CleanupPending(pendingSet); err != nil {
			logging.Warn("pending cleanup failed", "role", role, "err", err)
		}
	}
	if err := apply.RemoveRequest(reqPath); err != nil {
		logging.Warn("apply request cleanup failed", "path", reqPath, "err", err)
	}
	if applied {
		logging.Info("pending config applied", "role", role, "request_id", req.ID)
	} else {
		logging.Warn("apply request skipped (no pending data)", "role", role, "request_id", req.ID)
	}
	return rollback, applied, nil
}

func fileExists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
