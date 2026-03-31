package servercmd

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/cli/stateview"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/heartbeat"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

type serverStateOptions struct {
	Path     string
	Watch    bool
	Interval time.Duration
	TTL      time.Duration
}

const defaultHeartbeatTTL = 10 * time.Second

func newServerStateCmd(cfg commandConfig) *cobra.Command {
	opts := serverStateOptions{
		Interval: 2 * time.Second,
		TTL:      defaultHeartbeatTTL,
	}
	cmd := &cobra.Command{
		Use:   "state",
		Short: "Show heartbeat status for xp2p tunnels",
		RunE: func(cmd *cobra.Command, _ []string) error {
			code := runServerState(commandContext(cmd), cfg(), opts)
			return errorForCode(code)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.Path, "path", "p", "", "server installation directory")
	flags.BoolVarP(&opts.Watch, "watch", "w", false, "continuously refresh state until interrupted")
	flags.DurationVarP(&opts.Interval, "interval", "i", opts.Interval, "refresh interval for --watch")
	flags.DurationVarP(&opts.TTL, "ttl", "T", opts.TTL, "heartbeat TTL for alive status")
	return cmd
}

func runServerState(ctx context.Context, cfg config.Config, opts serverStateOptions) int {
	installDir := strings.TrimSpace(firstNonEmpty(opts.Path, cfg.Server.InstallDir))
	if installDir == "" {
		logging.Error("xp2p server state: install directory is required (use --path or configure server.install_dir)")
		return 2
	}
	installed, err := serverStateInstallPresent(installDir)
	if err != nil {
		logging.Error("xp2p server state: failed to check install status", "err", err)
		return 1
	}
	if !installed {
		logging.Info("xp2p server state: server is not installed")
		return 1
	}
	stateRoot := installDir
	if runtime.GOOS == "windows" {
		stateRoot = config.ConfigRoot()
	}
	statePath := filepath.Join(stateRoot, layout.ServerHeartbeatStateFileName)
	ttl := opts.TTL
	if ttl <= 0 {
		ttl = defaultHeartbeatTTL
	}
	interval := opts.Interval
	if interval <= 0 {
		interval = 2 * time.Second
	}

	if opts.Watch {
		err := stateview.WatchWithSnapshots(ctx, func() ([]heartbeat.Snapshot, error) {
			return snapshotServerState(statePath, installDir, ttl)
		}, interval)
		if err != nil && !errors.Is(err, context.Canceled) {
			logging.Error("xp2p server state: watch failed", "err", err)
			return 1
		}
		return 0
	}

	if err := stateview.PrintWithSnapshots(func() ([]heartbeat.Snapshot, error) {
		return snapshotServerState(statePath, installDir, ttl)
	}); err != nil {
		logging.Error("xp2p server state: failed to render state", "err", err)
		return 1
	}
	return 0
}

func serverStateInstallPresent(installDir string) (bool, error) {
	configPath := filepath.Clean(config.ConfigPath(layout.ServerConfigFileName))
	if found, err := pathExists(configPath); err != nil {
		return false, err
	} else if found {
		return true, nil
	}
	pendingPath := filepath.Clean(config.PendingConfigPath(layout.ServerConfigFileName))
	if found, err := pathExists(pendingPath); err != nil {
		return false, err
	} else if found {
		return true, nil
	}
	stateRoot := installDir
	if runtime.GOOS == "windows" {
		stateRoot = config.ConfigRoot()
	}
	for _, name := range []string{layout.ServerStateFileName, layout.StateFileName} {
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

func snapshotServerState(statePath, installDir string, ttl time.Duration) ([]heartbeat.Snapshot, error) {
	reversePairs, err := loadServerReversePairs(installDir)
	if err != nil {
		return nil, err
	}
	if len(reversePairs) == 0 {
		return []heartbeat.Snapshot{}, nil
	}
	snapshots, err := stateview.Snapshot(statePath, ttl)
	if err != nil {
		return nil, err
	}
	filtered := make([]heartbeat.Snapshot, 0, len(snapshots))
	for _, snap := range snapshots {
		user := strings.TrimSpace(snap.Entry.User)
		host := strings.TrimSpace(snap.Entry.Host)
		if user == "" || host == "" {
			continue
		}
		key := strings.ToLower(user) + "|" + strings.ToLower(host)
		if _, ok := reversePairs[key]; ok {
			filtered = append(filtered, snap)
		}
	}
	return filtered, nil
}

func loadServerReversePairs(installDir string) (map[string]struct{}, error) {
	configPath := filepath.Clean(config.ConfigPath(layout.ServerConfigFileName))
	if doc, found, err := loadServerConfigDoc(configPath); err != nil {
		return nil, err
	} else if found {
		return extractServerReversePairs(doc), nil
	}

	legacyDoc, err := loadLegacyServerStateDoc(installDir)
	if err != nil {
		return nil, err
	}
	return extractServerReversePairs(legacyDoc), nil
}

func loadServerConfigDoc(path string) (map[string]any, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return map[string]any{}, true, nil
	}
	tree, err := toml.LoadBytes(data)
	if err != nil {
		return nil, false, err
	}
	raw := tree.GetPath([]string{"server"})
	if raw == nil {
		return map[string]any{}, true, nil
	}
	switch value := raw.(type) {
	case *toml.Tree:
		raw = value.ToMap()
	case map[string]any:
		raw = value
	}
	doc, ok := raw.(map[string]any)
	if !ok {
		return map[string]any{}, true, nil
	}
	return doc, true, nil
}

func loadLegacyServerStateDoc(installDir string) (map[string]any, error) {
	stateRoot := installDir
	if runtime.GOOS == "windows" {
		stateRoot = config.ConfigRoot()
	}
	candidates := []string{
		filepath.Join(stateRoot, layout.ServerStateFileName),
		filepath.Join(stateRoot, layout.StateFileName),
	}
	var doc map[string]any
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		if len(strings.TrimSpace(string(data))) == 0 {
			continue
		}
		if err := json.Unmarshal(data, &doc); err != nil {
			return nil, err
		}
		break
	}
	if len(doc) == 0 {
		return map[string]any{}, nil
	}
	return doc, nil
}

func extractServerReversePairs(doc map[string]any) map[string]struct{} {
	if len(doc) == 0 {
		return map[string]struct{}{}
	}
	raw := doc["reverse_channels"]
	channels, ok := raw.(map[string]any)
	if !ok || len(channels) == 0 {
		return map[string]struct{}{}
	}
	pairs := make(map[string]struct{}, len(channels))
	for _, value := range channels {
		entry, ok := value.(map[string]any)
		if !ok {
			continue
		}
		user, _ := entry["user_id"].(string)
		host, _ := entry["host"].(string)
		user = strings.TrimSpace(user)
		host = strings.TrimSpace(host)
		if user == "" || host == "" {
			continue
		}
		key := strings.ToLower(user) + "|" + strings.ToLower(host)
		pairs[key] = struct{}{}
	}
	return pairs
}
