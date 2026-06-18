package clientcmd

import (
	"context"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

type clientEndpointUpdateOptions struct {
	Path        string
	ConfigDir   string
	Target      string
	User        string
	Password    string
	UserSet     bool
	PasswordSet bool
}

func runClientEndpointUpdate(ctx context.Context, cfg config.Config, opts clientEndpointUpdateOptions) int {
	if !opts.UserSet && !opts.PasswordSet {
		logging.Error("xp2p client update: at least one of --user or --password is required")
		return 2
	}
	if opts.UserSet && strings.TrimSpace(opts.User) == "" {
		logging.Error("xp2p client update: --user must not be empty")
		return 2
	}
	if opts.PasswordSet && strings.TrimSpace(opts.Password) == "" {
		logging.Error("xp2p client update: --password must not be empty")
		return 2
	}

	updateOpts := client.UpdateEndpointOptions{
		InstallDir:  firstNonEmpty(opts.Path, cfg.Client.InstallDir),
		ConfigDir:   firstNonEmpty(opts.ConfigDir, cfg.Client.ConfigDir),
		Target:      opts.Target,
		User:        opts.User,
		Password:    opts.Password,
		UserSet:     opts.UserSet,
		PasswordSet: opts.PasswordSet,
	}
	if err := clientUpdateEndpointFunc(ctx, updateOpts); err != nil {
		logging.Error("xp2p client update failed", "err", err)
		return 1
	}
	logging.Info("xp2p client endpoint updated", "target", strings.TrimSpace(opts.Target))
	return 0
}
