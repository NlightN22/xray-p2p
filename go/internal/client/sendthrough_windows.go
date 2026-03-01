//go:build windows

package client

import (
	"context"
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/winnet"
)

func updateSendThroughOutbound(ctx context.Context, paths clientPaths) error {
	sendThrough, err := winnet.DefaultSendThroughIPv4(ctx)
	if err != nil {
		logging.Warn("xp2p: failed to detect sendThrough IPv4", "err", err)
		sendThrough = ""
	} else if sendThrough == "" {
		logging.Warn("xp2p: sendThrough IPv4 not detected, leaving default outbound binding")
	}

	xrayCfg, err := ensureClientXrayConfig(paths.configFile)
	if err != nil {
		return err
	}
	state, err := loadClientInstallState(paths.configFile)
	if err != nil {
		return err
	}
	xrayCfg.DirectOutbound.SendThrough = sendThrough
	return writeOutboundsConfig(filepath.Join(paths.configDir, "outbounds.json"), xrayCfg.DirectOutbound, state.Endpoints)
}
