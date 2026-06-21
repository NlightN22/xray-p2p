package clientcmd

import (
	"context"

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
	bgCtx, cancel := context.WithCancel(ctx)
	liveDir, err := config.LiveRoleDir(apply.RoleClient)
	if err != nil {
		cancel()
		logging.Warn("xp2p diagnostics: failed to resolve live config", "port", port, "err", err)
		return nil
	}
	if err := server.StartBackground(bgCtx, server.Options{Port: port, LiveDir: liveDir}); err != nil {
		cancel()
		logging.Warn("xp2p diagnostics: failed to start ping responders", "port", port, "err", err)
		return nil
	}
	return cancel
}
