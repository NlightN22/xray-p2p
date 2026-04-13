//go:build windows

package client

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/winnet"
)

func updateSendThroughOutbound(ctx context.Context, paths clientPaths, tunEnabled bool) error {
	sendThrough := ""
	existingSendThrough := ""
	if tunEnabled {
		value, err := winnet.DefaultSendThroughIPv4(ctx)
		if err != nil {
			logging.Warn("failed to detect sendThrough IPv4", "err", err)
			return nil
		} else if value == "" {
			logging.Warn("sendThrough IPv4 not detected, leaving default outbound binding")
			return nil
		}
		sendThrough = value
	}

	xrayCfg, err := ensureClientXrayConfig(paths.configFile)
	if err != nil {
		return err
	}
	if directTag := strings.TrimSpace(xrayCfg.DirectOutbound.Tag); directTag != "" {
		existingSendThrough = existingOutboundSendThrough(filepath.Join(paths.configDir, "outbounds.json"), directTag)
	}
	if sendThrough == "" {
		sendThrough = existingSendThrough
	}
	if strings.EqualFold(sendThrough, existingSendThrough) {
		return nil
	}
	state, err := loadClientInstallState(paths.configFile)
	if err != nil {
		return err
	}
	xrayCfg.DirectOutbound.SendThrough = sendThrough
	return writeOutboundsConfig(filepath.Join(paths.configDir, "outbounds.json"), xrayCfg.DirectOutbound, state.Endpoints, nil, false)
}

func existingOutboundSendThrough(path string, directTag string) string {
	tag := strings.ToLower(strings.TrimSpace(directTag))
	if tag == "" {
		return ""
	}
	existing := readExistingOutbounds(path)
	for _, raw := range existing {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		outboundTag, ok := entry["tag"].(string)
		if !ok || strings.ToLower(strings.TrimSpace(outboundTag)) != tag {
			continue
		}
		value, _ := entry["sendThrough"].(string)
		return strings.TrimSpace(value)
	}
	return ""
}
