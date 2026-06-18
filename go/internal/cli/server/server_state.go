package servercmd

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/cli/stateview"
	"github.com/NlightN22/xray-p2p/go/internal/cli/xraystate"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/heartbeat"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

type serverStateOptions struct {
	Path        string
	Pending     bool
	Watch       bool
	Interval    time.Duration
	TTL         time.Duration
	XrayStats   bool
	XrayAPI     string
	XrayBin     string
	StatsFormat string
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
	flags.BoolVarP(&opts.Pending, "pending", "y", false, "show pending configuration")
	flags.BoolVarP(&opts.Watch, "watch", "w", false, "continuously refresh state until interrupted")
	flags.DurationVarP(&opts.Interval, "interval", "i", opts.Interval, "refresh interval for --watch")
	flags.DurationVarP(&opts.TTL, "ttl", "T", opts.TTL, "heartbeat TTL for alive status")
	flags.BoolVarP(&opts.XrayStats, "xray-stats", "X", false, "show Xray user traffic counters")
	flags.StringVarP(&opts.XrayAPI, "xray-api", "A", "", "Xray API address for stats")
	flags.StringVarP(&opts.XrayBin, "xray-bin", "B", "", "Xray binary path for statsquery")
	flags.StringVarP(&opts.StatsFormat, "xray-stats-format", "F", "human", "Xray stats format (human|bytes)")
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
	statePath, err := resolveServerHeartbeatStatePath(installDir)
	if err != nil {
		logging.Error("xp2p server state: resolve heartbeat path failed", "err", err)
		return 1
	}
	ttl := opts.TTL
	if ttl <= 0 {
		ttl = defaultHeartbeatTTL
	}
	interval := opts.Interval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	stateProvider := func() ([]heartbeat.Snapshot, error) {
		if opts.Pending {
			return snapshotServerPendingState(statePath, installDir, ttl)
		}
		return snapshotServerState(statePath, installDir, ttl)
	}
	viewProvider, err := xraystate.BuildViewProvider(ctx, stateProvider, xraystate.Options{
		Enabled:    opts.XrayStats,
		Role:       apply.RoleServer,
		InstallDir: installDir,
		APIAddress: opts.XrayAPI,
		XrayBin:    opts.XrayBin,
		Format:     opts.StatsFormat,
	})
	if err != nil {
		logging.Error("xp2p server state: invalid Xray stats options", "err", err)
		return 2
	}

	if opts.Watch {
		err := stateview.WatchWithView(ctx, viewProvider, interval)
		if err != nil && !errors.Is(err, context.Canceled) {
			logging.Error("xp2p server state: watch failed", "err", err)
			return 1
		}
		return 0
	}

	if err := stateview.PrintWithView(viewProvider); err != nil {
		logging.Error("xp2p server state: failed to render state", "err", err)
		return 1
	}
	return 0
}

type serverReverseEntry struct {
	Tag  string
	User string
	Host string
}

func serverStateInstallPresent(installDir string) (bool, error) {
	configPath := filepath.Clean(config.ConfigPath(layout.ServerConfigFileName))
	if found, err := pathExists(configPath); err != nil {
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
	return snapshotServerConfiguredState(statePath, installDir, ttl)
}

func resolveServerHeartbeatStatePath(installDir string) (string, error) {
	stateRoot := installDir
	if runtime.GOOS == "windows" {
		stateRoot = config.ConfigRoot()
	}
	livePath := filepath.Join(stateRoot, layout.ServerHeartbeatStateFileName)
	if found, err := pathExists(livePath); err != nil {
		return "", err
	} else if found {
		return livePath, nil
	}
	return livePath, nil
}

func loadServerReverseEntries(installDir string) ([]serverReverseEntry, error) {
	configPath := filepath.Clean(config.ConfigPath(layout.ServerConfigFileName))
	if doc, found, err := loadServerConfigDoc(configPath); err != nil {
		return nil, err
	} else if found {
		return extractServerReverseEntries(doc), nil
	}

	legacyDoc, err := loadLegacyServerStateDoc(installDir)
	if err != nil {
		return nil, err
	}
	return extractServerReverseEntries(legacyDoc), nil
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

func extractServerReverseEntries(doc map[string]any) []serverReverseEntry {
	if len(doc) == 0 {
		return nil
	}
	raw := doc["reverse_channels"]
	channels, ok := raw.(map[string]any)
	if !ok || len(channels) == 0 {
		return nil
	}
	entries := make([]serverReverseEntry, 0, len(channels))
	for tag, value := range channels {
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
		entries = append(entries, serverReverseEntry{
			Tag:  strings.TrimSpace(tag),
			User: user,
			Host: host,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].Tag) < strings.ToLower(entries[j].Tag)
	})
	return entries
}

func snapshotServerPendingState(statePath, installDir string, ttl time.Duration) ([]heartbeat.Snapshot, error) {
	return snapshotServerConfiguredState(statePath, installDir, ttl)
}

func snapshotServerConfiguredState(statePath, installDir string, ttl time.Duration) ([]heartbeat.Snapshot, error) {
	expected, err := loadServerExpectedEntries(installDir)
	if err != nil {
		return nil, err
	}
	if len(expected) == 0 {
		return []heartbeat.Snapshot{}, nil
	}

	snapshots, err := stateview.Snapshot(statePath, ttl)
	if err != nil {
		return nil, err
	}

	byUserHost := make(map[string]heartbeat.Snapshot, len(snapshots))
	byUser := make(map[string]heartbeat.Snapshot, len(snapshots))
	for _, snap := range snapshots {
		user := strings.TrimSpace(snap.Entry.User)
		host := strings.TrimSpace(snap.Entry.Host)
		if user == "" {
			continue
		}
		userKey := strings.ToLower(user)
		if _, exists := byUser[userKey]; !exists {
			byUser[userKey] = snap
		}
		if host != "" {
			key := userKey + "|" + strings.ToLower(host)
			byUserHost[key] = snap
		}
	}

	merged := make([]heartbeat.Snapshot, 0, len(expected))
	for _, exp := range expected {
		key := strings.ToLower(exp.User) + "|" + strings.ToLower(exp.Host)
		snap, ok := byUserHost[key]
		if !ok && strings.TrimSpace(exp.Host) == "-" {
			snap, ok = byUser[strings.ToLower(exp.User)]
		}
		if ok {
			if strings.TrimSpace(snap.Entry.Tag) == "" && strings.TrimSpace(exp.Tag) != "" {
				snap.Entry.Tag = exp.Tag
			}
			merged = append(merged, snap)
			continue
		}
		tag := strings.TrimSpace(exp.Tag)
		if tag == "" {
			tag = "-"
		}
		merged = append(merged, heartbeat.Snapshot{
			Entry: heartbeat.Entry{
				Tag:  tag,
				Host: exp.Host,
				User: exp.User,
			},
			AvgRTTMillis: 0,
			Alive:        false,
			Age:          0,
		})
	}

	sort.Slice(merged, func(i, j int) bool {
		leftTag := strings.ToLower(merged[i].Entry.Tag)
		rightTag := strings.ToLower(merged[j].Entry.Tag)
		if leftTag == rightTag {
			leftHost := strings.ToLower(merged[i].Entry.Host)
			rightHost := strings.ToLower(merged[j].Entry.Host)
			if leftHost == rightHost {
				leftUser := strings.ToLower(merged[i].Entry.User)
				rightUser := strings.ToLower(merged[j].Entry.User)
				if leftUser == rightUser {
					return strings.ToLower(merged[i].Entry.ClientIP) < strings.ToLower(merged[j].Entry.ClientIP)
				}
				return leftUser < rightUser
			}
			return leftHost < rightHost
		}
		return leftTag < rightTag
	})
	return merged, nil
}

func loadServerExpectedEntries(installDir string) ([]serverReverseEntry, error) {
	configPath := filepath.Clean(config.ConfigPath(layout.ServerConfigFileName))
	if doc, found, err := loadServerConfigDoc(configPath); err != nil {
		return nil, err
	} else if found {
		return extractServerExpectedEntries(doc), nil
	}

	legacyDoc, err := loadLegacyServerStateDoc(installDir)
	if err != nil {
		return nil, err
	}
	return extractServerExpectedEntries(legacyDoc), nil
}

func extractServerExpectedEntries(doc map[string]any) []serverReverseEntry {
	users := extractServerUserIDs(doc)
	reverseEntries := extractServerReverseEntries(doc)
	if len(users) == 0 {
		return reverseEntries
	}

	entries := make([]serverReverseEntry, 0, len(users))
	byUser := make(map[string]int, len(users))
	for _, user := range users {
		key := strings.ToLower(user)
		if _, exists := byUser[key]; exists {
			continue
		}
		byUser[key] = len(entries)
		entries = append(entries, serverReverseEntry{Tag: "-", User: user, Host: "-"})
	}
	for _, reverse := range reverseEntries {
		key := strings.ToLower(strings.TrimSpace(reverse.User))
		idx, exists := byUser[key]
		if !exists {
			continue
		}
		if strings.TrimSpace(entries[idx].Tag) == "-" && strings.TrimSpace(reverse.Tag) != "" {
			entries[idx].Tag = reverse.Tag
		}
		if strings.TrimSpace(entries[idx].Host) == "-" && strings.TrimSpace(reverse.Host) != "" {
			entries[idx].Host = reverse.Host
		}
	}
	return entries
}

func extractServerUserIDs(doc map[string]any) []string {
	raw, ok := doc["trojan_users"].([]any)
	if !ok || len(raw) == 0 {
		return nil
	}
	users := make([]string, 0, len(raw))
	for _, value := range raw {
		entry, ok := value.(map[string]any)
		if !ok {
			continue
		}
		user, _ := entry["email"].(string)
		user = strings.TrimSpace(user)
		if user == "" {
			continue
		}
		users = append(users, user)
	}
	return users
}
