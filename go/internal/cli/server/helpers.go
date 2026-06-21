package servercmd

import (
	"context"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func startDiagnostics(ctx context.Context, port, installDir string) context.CancelFunc {
	bgCtx, cancel := context.WithCancel(ctx)
	liveDir, err := config.LiveRoleDir(apply.RoleServer)
	if err != nil {
		cancel()
		logging.Warn("xp2p diagnostics: failed to resolve live config", "port", port, "err", err)
		return nil
	}
	if err := server.StartBackground(bgCtx, server.Options{Port: port, InstallDir: installDir, LiveDir: liveDir}); err != nil {
		cancel()
		logging.Warn("xp2p diagnostics: failed to start ping responders", "port", port, "err", err)
		return nil
	}
	return cancel
}
