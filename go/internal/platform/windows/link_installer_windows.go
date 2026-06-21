//go:build windows

package windows

import (
	"context"
	"fmt"
	"strconv"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
)

type LinkInstaller struct{}

func NewLinkInstaller() *LinkInstaller {
	return &LinkInstaller{}
}

func (l *LinkInstaller) Install(ctx context.Context, rawLink string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	cfg, err := config.Load(config.Options{})
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	parsed, err := tunnel.ParseLink(rawLink)
	if err != nil {
		return err
	}
	if parsed.Endpoint.Protocol != "trojan" {
		return fmt.Errorf("connection link protocol %q is not trojan", parsed.Endpoint.Protocol)
	}
	if parsed.User.UserLabel == "" {
		return fmt.Errorf("connection link missing user label (expected #label or email/remarks query parameter)")
	}

	opts, err := buildInstallOptionsFromLink(cfg, parsed)
	if err != nil {
		return err
	}

	return client.Install(ctx, opts)
}

func buildInstallOptionsFromLink(cfg config.Config, link tunnel.Link) (client.InstallOptions, error) {
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
	}, nil
}
