package clientcmd

import (
	"context"

	clishared "github.com/NlightN22/xray-p2p/go/internal/cli/common"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

var promptYesNoFunc = clishared.PromptYesNo

func firstNonEmpty(values ...string) string {
	return clishared.FirstNonEmpty(values...)
}

func startDiagnostics(ctx context.Context, port string) context.CancelFunc {
	bgCtx, cancel := context.WithCancel(ctx)
	if err := server.StartBackground(bgCtx, server.Options{Port: port}); err != nil {
		cancel()
		logging.Warn("xp2p diagnostics: failed to start ping responders", "port", port, "err", err)
		return nil
	}
	return cancel
}
