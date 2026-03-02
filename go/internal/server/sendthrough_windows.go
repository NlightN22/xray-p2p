//go:build windows

package server

import (
	"context"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/winnet"
)

func updateSendThroughOutbound(ctx context.Context, configDir string, tunEnabled bool) error {
	sendThrough := ""
	if tunEnabled {
		value, err := winnet.DefaultSendThroughIPv4(ctx)
		if err != nil {
			logging.Warn("xp2p: failed to detect sendThrough IPv4", "err", err)
		} else if value == "" {
			logging.Warn("xp2p: sendThrough IPv4 not detected, leaving default outbound binding")
		} else {
			sendThrough = value
		}
	}

	xrayCfg, err := ensureServerXrayConfig(config.ConfigPath(layout.ServerConfigFileName))
	if err != nil {
		return err
	}
	xrayCfg.DirectOutbound.SendThrough = sendThrough
	return writeServerOutbounds(configDir, xrayCfg.DirectOutbound)
}
