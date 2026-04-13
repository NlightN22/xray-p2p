//go:build windows

package client

import (
	"context"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func restoreFullTunnelOnStop(installDir, configDirName string) {
	if windowsRoutesDisabled {
		logging.Info("windows route apply disabled; skipping full-tunnel restore on stop")
		return
	}
	paths, err := resolveClientPaths(installDir, configDirName)
	if err != nil {
		logging.Warn("full-tunnel restore on stop skipped", "err", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := restoreFullTunnel(ctx, paths, false); err != nil {
		logging.Warn("full-tunnel restore on stop failed", "err", err)
	}
}
