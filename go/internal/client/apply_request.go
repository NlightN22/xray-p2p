//go:build linux || windows

package client

import (
	"os"
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func applyPendingIfRequested(role string, configDir string) (*apply.Rollback, bool, apply.Request, error) {
	pendingConfigDir, err := config.PendingConfigDir(configDir)
	if err != nil {
		return nil, false, apply.Request{}, err
	}
	liveConfigDir, err := config.LiveConfigDir(configDir)
	if err != nil {
		return nil, false, apply.Request{}, err
	}
	reqPath := config.ApplyRequestPath()
	logging.Debug("apply request check",
		"role", role,
		"apply_request", reqPath,
		"apply_request_exists", fileExists(reqPath),
		"state_root", config.StateRoot(),
		"state_root_exists", dirExists(config.StateRoot()),
		"pending_root", config.PendingRoot(),
		"pending_root_exists", dirExists(config.PendingRoot()),
		"pending_config", config.PendingConfigPath(layout.ClientConfigFileName),
		"pending_config_exists", fileExists(config.PendingConfigPath(layout.ClientConfigFileName)),
		"pending_dir", pendingConfigDir,
		"pending_dir_exists", dirExists(pendingConfigDir),
		"live_config", config.LiveConfigPath(layout.ClientConfigFileName),
		"live_config_exists", fileExists(config.LiveConfigPath(layout.ClientConfigFileName)),
		"live_config_dir", liveConfigDir,
		"live_config_dir_exists", dirExists(liveConfigDir),
	)
	req, exists, err := apply.ReadRequest(reqPath)
	if err != nil {
		return nil, false, apply.Request{}, err
	}
	if !exists {
		return nil, false, apply.Request{}, nil
	}
	if !req.MatchesRole(role) {
		return nil, false, apply.Request{}, nil
	}
	errorPath := config.ApplyErrorPath()
	if marker, markerExists, err := apply.ReadError(errorPath); err != nil {
		return nil, false, apply.Request{}, err
	} else if markerExists && marker.RequestID != "" && marker.RequestID == req.ID {
		logging.Warn("apply request skipped (previous failure)", "role", role, "request_id", req.ID, "reason", marker.Reason)
		return nil, false, req, nil
	}
	pendingSet := apply.PendingSet{
		LiveConfigFile:    filepath.Clean(config.LiveConfigPath(layout.ClientConfigFileName)),
		PendingConfigFile: filepath.Clean(config.PendingConfigPath(layout.ClientConfigFileName)),
		LiveConfigDir:     liveConfigDir,
		PendingConfigDir:  pendingConfigDir,
		LiveRoot:          config.LiveRoot(),
		LkgRoot:           config.LkgRoot(),
		AuditPath:         config.AuditLogPath(),
	}
	rollback, applied, err := apply.ApplyPending(pendingSet)
	if err != nil {
		if cleanupErr := apply.CleanupPending(pendingSet); cleanupErr != nil {
			logging.Warn("pending cleanup failed after apply error", "role", role, "err", cleanupErr)
		}
		_ = apply.WriteError(errorPath, apply.ErrorMarker{
			RequestID: req.ID,
			Role:      role,
			Reason:    err.Error(),
		}, config.AuditLogPath())
		logging.Warn("pending apply failed; rollback restored live config", "role", role, "request_id", req.ID, "err", err)
		return nil, false, req, nil
	}
	if !applied {
		if err := apply.RemoveRequest(reqPath); err != nil {
			logging.Warn("apply request cleanup failed", "path", reqPath, "err", err)
		}
		if err := apply.RemoveError(errorPath); err != nil {
			logging.Warn("apply error cleanup failed", "path", errorPath, "err", err)
		}
		logging.Warn("apply request skipped (no pending data)", "role", role, "request_id", req.ID)
		return nil, false, req, nil
	}
	if applied {
		if err := apply.CleanupPending(pendingSet); err != nil {
			logging.Warn("pending cleanup failed", "role", role, "err", err)
		}
	}
	logging.Info("pending config applied", "role", role, "request_id", req.ID)
	return rollback, applied, req, nil
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
