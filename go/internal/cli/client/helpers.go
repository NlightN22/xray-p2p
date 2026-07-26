package clientcmd

import (
	"context"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	clishared "github.com/NlightN22/xray-p2p/go/internal/cli/common"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

var promptYesNoFunc = clishared.PromptYesNo

func firstNonEmpty(values ...string) string {
	return clishared.FirstNonEmpty(values...)
}

func startDiagnostics(ctx context.Context, port string) context.CancelFunc {
	liveDir, err := config.LiveRoleDir(apply.RoleClient)
	if err != nil {
		logging.Warn("xp2p diagnostics: failed to resolve live config", "port", port, "err", err)
		return nil
	}
	owner, err := server.StartBackground(ctx, server.Options{Port: port, LiveDir: liveDir})
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
