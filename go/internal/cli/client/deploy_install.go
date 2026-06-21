package clientcmd

import (
	"strconv"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
)

// buildInstallOptionsFromLink converts a parsed connection link into client install options,
// applying config defaults for install paths.
func buildInstallOptionsFromLink(cfg config.Config, link trojanLink) client.InstallOptions {
	endpoint := link.Endpoint
	allowInsecure := endpoint.TLS.AllowInsecure
	if endpoint.TLS.PinnedPeerCertSHA256 != "" {
		allowInsecure = false
	}
	return client.InstallOptions{
		InstallDir:           cfg.Client.InstallDir,
		ConfigDir:            cfg.Client.ConfigDir,
		ServerAddress:        endpoint.Host,
		ServerPort:           strconv.Itoa(endpoint.Port),
		User:                 link.User.UserLabel,
		Password:             tunnel.ActiveCredential(link.User),
		ServerName:           endpoint.ServerName,
		ALPN:                 endpoint.TLS.ALPN,
		AllowInsecure:        allowInsecure,
		PinnedPeerCertSHA256: endpoint.TLS.PinnedPeerCertSHA256,
		VerifyPeerCertByName: endpoint.TLS.VerifyPeerCertByName,
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
