package clientcmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
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
	flags.StringVarP(&opts.Path, "path", "p", "", "client installation directory")
	flags.BoolVarP(&opts.Watch, "watch", "w", false, "continuously refresh state until interrupted")
	flags.DurationVarP(&opts.Interval, "interval", "i", opts.Interval, "refresh interval for --watch")
	flags.DurationVarP(&opts.TTL, "ttl", "T", opts.TTL, "heartbeat TTL for alive status")
	return cmd
}

func runClientState(ctx context.Context, cfg config.Config, opts clientStateOptions) int {
	installDir := strings.TrimSpace(firstNonEmpty(opts.Path, cfg.Client.InstallDir))
	if installDir == "" {
		logging.Error("xp2p client state: install directory is required (use --path or configure client.install_dir)")
		return 2
	}
	installed, err := clientStateInstallPresent(installDir)
	if err != nil {
		logging.Error("xp2p client state: failed to check install status", "err", err)
		return 1
	}
	if !installed {
		logging.Info("xp2p client state: client is not installed")
		return 1
	}
	stateRoot := installDir
	if runtime.GOOS == "windows" {
		stateRoot = config.ConfigRoot()
	}
	statePath := filepath.Join(stateRoot, layout.ClientHeartbeatStateFileName)
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

func clientStateInstallPresent(installDir string) (bool, error) {
	configPath := filepath.Clean(config.ConfigPath(layout.ClientConfigFileName))
	if found, err := pathExists(configPath); err != nil {
		return false, err
	} else if found {
		return true, nil
	}
	pendingState := filepath.Clean(config.ConfigPath(layout.ClientAppliedStateFileName))
	if found, err := pathExists(pendingState); err != nil {
		return false, err
	} else if found {
		return true, nil
	}
	stateRoot := installDir
	if runtime.GOOS == "windows" {
		stateRoot = config.ConfigRoot()
	}
	for _, name := range []string{layout.ClientStateFileName, layout.StateFileName} {
		path := filepath.Join(stateRoot, name)
		if found, err := pathExists(path); err != nil {
			return false, err
		} else if found {
			return true, nil
		}
	}
	return false, nil
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func snapshotClientState(installDir, configDir, statePath string, ttl time.Duration) ([]heartbeat.Snapshot, error) {
	state, err := heartbeat.Load(statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || heartbeat.IsCorrupt(err) {
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
		Pending:    false,
	})
	if err != nil {
		return nil, err
	}

	filtered := make(map[string]heartbeat.Entry, len(endpoints))
	for _, endpoint := range endpoints {
		key := heartbeatEntryKey(endpoint.Tag, endpoint.User)
		if key == "" {
			continue
		}
		entry, exists := state.Entries[key]
		if !exists {
			entry = heartbeat.Entry{}
		}
		if entry.Tag == "" {
			entry.Tag = endpoint.Tag
		}
		if entry.Host == "" {
			entry.Host = endpoint.Hostname
		}
		if entry.User == "" {
			entry.User = endpoint.User
		}
		filtered[key] = entry
	}

	state.Entries = filtered
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
