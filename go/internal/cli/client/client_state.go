package clientcmd

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/cli/stateview"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

type clientStateOptions struct {
	Path     string
	Watch    bool
	Interval time.Duration
	TTL      time.Duration
}

const defaultHeartbeatTTL = 10 * time.Second

func newClientStateCmd(cfg commandConfig) *cobra.Command {
	opts := clientStateOptions{
		Interval: 2 * time.Second,
		TTL:      defaultHeartbeatTTL,
	}
	cmd := &cobra.Command{
		Use:   "state",
		Short: "Show local heartbeat cache status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runClientState(commandContext(cmd), cfg(), opts)
			return errorForCode(code)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.Path, "path", "", "client installation directory")
	flags.BoolVar(&opts.Watch, "watch", false, "continuously refresh state until interrupted")
	flags.DurationVar(&opts.Interval, "interval", opts.Interval, "refresh interval for --watch")
	flags.DurationVar(&opts.TTL, "ttl", opts.TTL, "heartbeat TTL for alive status")
	return cmd
}

func runClientState(ctx context.Context, cfg config.Config, opts clientStateOptions) int {
	installDir := strings.TrimSpace(firstNonEmpty(opts.Path, cfg.Client.InstallDir))
	if installDir == "" {
		logging.Error("xp2p client state: install directory is required (use --path or configure client.install_dir)")
		return 2
	}
	statePath := filepath.Join(installDir, layout.HeartbeatStateFileName)
	ttl := opts.TTL
	if ttl <= 0 {
		ttl = defaultHeartbeatTTL
	}
	interval := opts.Interval
	if interval <= 0 {
		interval = 2 * time.Second
	}

	if opts.Watch {
		err := stateview.Watch(ctx, statePath, interval, ttl)
		if err != nil && !errors.Is(err, context.Canceled) {
			logging.Error("xp2p client state: watch failed", "err", err)
			return 1
		}
		return 0
	}

	if err := stateview.Print(statePath, ttl); err != nil {
		logging.Error("xp2p client state: failed to render state", "err", err)
		return 1
	}
	return 0
}
