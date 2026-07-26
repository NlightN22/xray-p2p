package servercmd

import (
	"context"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func startDiagnostics(ctx context.Context, port, installDir string) context.CancelFunc {
	liveDir, err := config.LiveRoleDir(apply.RoleServer)
	if err != nil {
		logging.Warn("xp2p diagnostics: failed to resolve live config", "port", port, "err", err)
		return nil
	}
	owner, err := server.StartBackground(ctx, server.Options{Port: port, InstallDir: installDir, LiveDir: liveDir})
	if err != nil {
		logging.Warn("xp2p diagnostics: failed to start ping responders", "port", port, "err", err)
		return nil
	}
	return func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		if err := owner.Stop(stopCtx); err != nil {
			logging.Warn("xp2p diagnostics shutdown failed", "err", err)
		}
	}
}
