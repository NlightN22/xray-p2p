//go:build linux || windows

package server

import (
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func applyPendingIfRequested(role string, configDir string) (*apply.Rollback, bool, error) {
	reqPath := config.ApplyRequestPath()
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
	if err := apply.RemoveRequest(reqPath); err != nil {
		logging.Warn("xp2p: apply request cleanup failed", "path", reqPath, "err", err)
	}
	if applied {
		logging.Info("xp2p: pending config applied", "role", role, "request_id", req.ID)
	} else {
		logging.Warn("xp2p: apply request skipped (no pending data)", "role", role, "request_id", req.ID)
	}
	return rollback, applied, nil
}
