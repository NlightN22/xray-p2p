package clientcmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/cli/stateview"
	"github.com/NlightN22/xray-p2p/go/internal/cli/xraystate"
	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/heartbeat"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

type clientStateOptions struct {
	Path          string
	Pending       bool
	Watch         bool
	Interval      time.Duration
	TTL           time.Duration
	XrayStats     bool
	XrayAPI       string
	XrayBin       string
	StatsFormat   string
	HealthDetails bool
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
	flags.BoolVarP(&opts.Pending, "pending", "y", false, "show pending configuration")
	flags.BoolVarP(&opts.Watch, "watch", "w", false, "continuously refresh state until interrupted")
	flags.DurationVarP(&opts.Interval, "interval", "i", opts.Interval, "refresh interval for --watch")
	flags.DurationVarP(&opts.TTL, "ttl", "T", opts.TTL, "heartbeat TTL for alive status")
	flags.BoolVarP(&opts.XrayStats, "xray-stats", "X", false, "show Xray user traffic counters")
	flags.StringVarP(&opts.XrayAPI, "xray-api", "A", "", "Xray API address for stats")
	flags.StringVarP(&opts.XrayBin, "xray-bin", "B", "", "deprecated; stats use direct Xray gRPC")
	flags.StringVarP(&opts.StatsFormat, "xray-stats-format", "F", "human", "Xray stats format (human|bytes)")
	flags.BoolVarP(&opts.HealthDetails, "health-details", "Z", false, "show heartbeat health diagnostic columns")
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
		return snapshotClientConfiguredState(installDir, configDir, opts.Pending, statePath, ttl)
	}
	viewProvider, err := xraystate.BuildViewProvider(ctx, stateProvider, xraystate.Options{
		Enabled:    opts.XrayStats,
		Role:       apply.RoleClient,
		InstallDir: installDir,
		APIAddress: opts.XrayAPI,
		XrayBin:    opts.XrayBin,
		Format:     opts.StatsFormat,
	})
	if err != nil {
		logging.Error("xp2p client state: invalid Xray stats options", "err", err)
		return 2
	}
	baseViewProvider := viewProvider
	viewProvider = func() (stateview.SnapshotView, error) {
		view, viewErr := baseViewProvider()
		view.ShowHealthDetails = opts.HealthDetails
		return view, viewErr
	}

	if opts.Watch {
		err := stateview.WatchWithView(ctx, viewProvider, interval)
		if err != nil && !errors.Is(err, context.Canceled) {
			logging.Error("xp2p client state: watch failed", "err", err)
			return 1
		}
		return 0
	}

	if err := stateview.PrintWithView(viewProvider); err != nil {
		logging.Error("xp2p client state: failed to render state", "err", err)
		return 1
	}
	if !opts.Pending {
		if err := printClientRuntimeStatus(config.ConfigPath(layout.ClientAppliedStateFileName)); err != nil {
			logging.Warn("client runtime state read failed", "err", err)
		}
	}
	return 0
}

func printClientRuntimeStatus(path string) error {
	status, err := client.LoadRuntimeStatus(path)
	if err != nil {
		return err
	}
	if status.Status == "" && !status.LoopProtectionSeen {
		return nil
	}

	fmt.Println()
	fmt.Println("RUNTIME_STATUS")
	if status.Status != "" {
		fmt.Printf("STATUS=%s\n", status.Status)
	}
	if status.Reason != "" {
		fmt.Printf("REASON=%s\n", status.Reason)
	}
	if status.RelatedOutbound != "" {
		fmt.Printf("RELATED_OUTBOUND=%s\n", status.RelatedOutbound)
	}
	if status.LoopProtectionSeen {
		fmt.Printf("FD_BEFORE=%d\n", status.FDBefore)
		fmt.Printf("FD_AFTER=%d\n", status.FDAfter)
		fmt.Printf("FD_DELTA=%d\n", status.FDDelta)
		if status.Window != "" {
			fmt.Printf("WINDOW=%s\n", status.Window)
		}
		if status.Action != "" {
			fmt.Printf("ACTION=%s\n", status.Action)
		}
		if !status.DetectedAt.IsZero() {
			fmt.Printf("DETECTED_AT=%s\n", status.DetectedAt.UTC().Format(time.RFC3339))
		}
	}
	return nil
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

func snapshotClientConfiguredState(installDir, configDir string, pending bool, statePath string, ttl time.Duration) ([]heartbeat.Snapshot, error) {
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
		Pending:    pending,
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
		entry, exists := state.Entries["v1|"+endpoint.HeartbeatID]
		if !exists {
			entry, exists = state.Entries[key]
		}
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
		entry.Mode = heartbeat.Mode(endpoint.HeartbeatMode)
		if entry.Mode == heartbeat.ModeDisabled {
			entry.Status = heartbeat.StatusDisabled
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
