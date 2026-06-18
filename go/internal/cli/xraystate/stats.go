package xraystate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/cli/stateview"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/heartbeat"
	"github.com/NlightN22/xray-p2p/go/internal/xraystats"
)

// Options controls optional Xray stats enrichment for state tables.
type Options struct {
	Enabled    bool
	Role       string
	InstallDir string
	APIAddress string
	XrayBin    string
	Format     string
}

// BuildViewProvider adds optional Xray traffic stats to a heartbeat provider.
func BuildViewProvider(ctx context.Context, snapshots stateview.SnapshotProvider, opts Options) (stateview.ViewProvider, error) {
	format, err := xraystats.NormalizeFormat(opts.Format)
	if err != nil {
		return nil, err
	}
	if !opts.Enabled {
		return func() (stateview.SnapshotView, error) {
			items, err := snapshots()
			if err != nil {
				return stateview.SnapshotView{}, err
			}
			return stateview.SnapshotView{Snapshots: items}, nil
		}, nil
	}
	apiAddress := strings.TrimSpace(opts.APIAddress)
	if apiAddress == "" {
		apiAddress = resolveAPIAddress(opts.Role)
	}
	return func() (stateview.SnapshotView, error) {
		items, err := snapshots()
		if err != nil {
			return stateview.SnapshotView{}, err
		}
		stats, err := xraystats.QueryUserStats(ctx, xraystats.QueryOptions{
			APIAddress: apiAddress,
			Timeout:    3 * time.Second,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to query Xray stats from %s: %v\n", apiAddress, err)
			return stateview.SnapshotView{
				Snapshots: items,
				Stats:     emptyStats(items),
				ShowStats: true,
			}, nil
		}
		return stateview.SnapshotView{
			Snapshots: items,
			Stats:     renderStats(items, stats, format),
			ShowStats: true,
		}, nil
	}, nil
}

func resolveAPIAddress(role string) string {
	path, err := config.LiveXrayPath(role)
	if err == nil {
		if value, readErr := xraystats.APIListenFromXrayConfig(path); readErr == nil && strings.TrimSpace(value) != "" {
			return value
		} else if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
			_ = readErr
		}
	}
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "server":
		return "127.0.0.1:52080"
	default:
		return "127.0.0.1:52180"
	}
}

func emptyStats(snapshots []heartbeat.Snapshot) map[string]stateview.TrafficStats {
	result := make(map[string]stateview.TrafficStats, len(snapshots))
	for _, snap := range snapshots {
		user := xraystats.NormalizeUser(snap.Entry.User)
		if user == "" {
			continue
		}
		result[user] = stateview.TrafficStats{Upload: "-", Download: "-", Total: "-"}
	}
	return result
}

func renderStats(snapshots []heartbeat.Snapshot, stats map[string]xraystats.TrafficStats, format string) map[string]stateview.TrafficStats {
	result := emptyStats(snapshots)
	for _, snap := range snapshots {
		user := xraystats.NormalizeUser(snap.Entry.User)
		if user == "" {
			continue
		}
		item, ok := stats[user]
		if !ok {
			continue
		}
		upload, download, total := xraystats.FormatTraffic(item, format)
		result[user] = stateview.TrafficStats{
			Upload:   upload,
			Download: download,
			Total:    total,
		}
	}
	return result
}
