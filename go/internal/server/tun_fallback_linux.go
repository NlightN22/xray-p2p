//go:build linux

package server

import (
	"fmt"

	"github.com/NlightN22/xray-p2p/go/internal/linuxnet"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func fallbackToProxyMode(tunEnabled *bool, err error, action string) bool {
	if !linuxnet.IsTunPermissionError(err) {
		return false
	}
	if tunEnabled != nil {
		*tunEnabled = false
	}
	logging.Warn("tun setup failed, switching to proxy mode", "action", action, "err", err)
	logging.Info("set XP2P_SERVER_TUN_ENABLED=false or run \"xp2p server mode proxy\" to keep proxy mode")
	return true
}

func tunSetupError(action string, err error) error {
	return fmt.Errorf("xp2p: tun setup failed during %s: %w", action, err)
}
