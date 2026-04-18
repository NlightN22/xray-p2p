package clientcmd

import (
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
)

// buildInstallOptionsFromLink converts a parsed trojan link into client install options,
// applying config defaults for install paths.
func buildInstallOptionsFromLink(cfg config.Config, link trojanLink) client.InstallOptions {
	allowInsecure := link.AllowInsecure
	if link.PinnedPeerSHA256 != "" {
		allowInsecure = false
	}
	return client.InstallOptions{
		InstallDir:           cfg.Client.InstallDir,
		ConfigDir:            cfg.Client.ConfigDir,
		ServerAddress:        link.ServerAddress,
		ServerPort:           link.ServerPort,
		User:                 link.User,
		Password:             link.Password,
		ServerName:           link.ServerName,
		ALPN:                 link.ALPN,
		AllowInsecure:        allowInsecure,
		PinnedPeerCertSHA256: link.PinnedPeerSHA256,
		VerifyPeerCertByName: link.VerifyPeerName,
		Force:                true,
		TunEnabled:           cfg.Client.TunEnabled,
		TunEnabledSet:        true,
		TunName:              cfg.Client.TunName,
		TunMTU:               cfg.Client.TunMTU,
		TunAddr:              cfg.Client.TunAddr,
		TunMode:              cfg.Client.TunMode,
	}
}

func applyClientDeployMode(installOpts client.InstallOptions, cfg config.Config, tunEnabled bool, tunMode string, tunModeSet bool, fullTunnelTag string) error {
	updatedPath, err := config.UpdateTunEnabled("", "client", tunEnabled)
	if err != nil {
		return err
	}
	if _, err := config.EnsureTunSettings("", "client", tunEnabled, cfg.Client.TunName, cfg.Client.TunMTU, cfg.Client.TunAddr); err != nil {
		return err
	}
	if tunModeSet {
		if _, err := config.UpdateTunMode("", "client", tunMode); err != nil {
			return err
		}
	}
	if tunModeSet && strings.EqualFold(strings.TrimSpace(tunMode), "full") && strings.TrimSpace(fullTunnelTag) != "" {
		if _, err := config.UpdateFullTunnelTag("", fullTunnelTag); err != nil {
			return err
		}
	}
	logDeployPaths("xp2p client deploy: mode config updated", updatedPath)
	req, err := apply.NewRequest(apply.RoleClient)
	if err != nil {
		return err
	}
	if err := apply.WriteRequest(config.ApplyRequestPath(), req, config.AuditLogPath()); err != nil {
		return err
	}
	logDeployPaths("xp2p client deploy: apply request written", updatedPath)
	return nil
}
