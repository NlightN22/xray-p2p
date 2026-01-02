package clientcmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/cli/stateview"
	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/heartbeat"
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

	configDir := strings.TrimSpace(cfg.Client.ConfigDir)
	stateProvider := func() ([]heartbeat.Snapshot, error) {
		return snapshotClientState(installDir, configDir, statePath, ttl)
	}

	if opts.Watch {
		err := stateview.WatchWithSnapshots(ctx, stateProvider, interval)
		if err != nil && !errors.Is(err, context.Canceled) {
			logging.Error("xp2p client state: watch failed", "err", err)
			return 1
		}
		return 0
	}

	if err := stateview.PrintWithSnapshots(stateProvider); err != nil {
		logging.Error("xp2p client state: failed to render state", "err", err)
		return 1
	}
	return 0
}

func snapshotClientState(installDir, configDir, statePath string, ttl time.Duration) ([]heartbeat.Snapshot, error) {
	state, err := heartbeat.Load(statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			state = heartbeat.State{}
		} else {
			return nil, err
		}
	}
	if state.Entries == nil {
		state.Entries = make(map[string]heartbeat.Entry)
	}

	endpoints, err := client.ListEndpoints(client.ListOptions{
		InstallDir: installDir,
		ConfigDir:  configDir,
	})
	if err != nil {
		return nil, err
	}

	for _, endpoint := range endpoints {
		key := heartbeatEntryKey(endpoint.Tag, endpoint.User)
		if key == "" {
			continue
		}
		entry, exists := state.Entries[key]
		if !exists {
			state.Entries[key] = heartbeat.Entry{
				Tag:  endpoint.Tag,
				Host: endpoint.Hostname,
				User: endpoint.User,
			}
			continue
		}
		if entry.Host == "" {
			entry.Host = endpoint.Hostname
		}
		if entry.User == "" {
			entry.User = endpoint.User
		}
		state.Entries[key] = entry
	}

	return state.Snapshot(time.Now(), ttl), nil
}

func heartbeatEntryKey(tag, user string) string {
	trimmedTag := strings.ToLower(strings.TrimSpace(tag))
	if trimmedTag == "" {
		return ""
	}
	trimmedUser := strings.ToLower(strings.TrimSpace(user))
	if trimmedUser == "" {
		return trimmedTag
	}
	return trimmedTag + "|" + trimmedUser
}
