//go:build windows

package windows

import (
	"context"
	"fmt"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/link"
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

	parsed, err := link.ParseTrojanLink(rawLink)
	if err != nil {
		return err
	}

	opts, err := buildInstallOptionsFromLink(cfg, parsed)
	if err != nil {
		return err
	}

	return client.Install(ctx, opts)
}

func buildInstallOptionsFromLink(cfg config.Config, link link.TrojanLink) (client.InstallOptions, error) {
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
		AllowInsecure:        allowInsecure,
		PinnedPeerCertSHA256: link.PinnedPeerSHA256,
		VerifyPeerCertByName: link.VerifyPeerName,
		Force:                true,
		TunEnabled:           cfg.Client.TunEnabled,
		TunEnabledSet:        true,
		TunName:              cfg.Client.TunName,
		TunMTU:               cfg.Client.TunMTU,
		TunAddr:              cfg.Client.TunAddr,
	}, nil
}
