package stateview

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/heartbeat"
)

// SnapshotProvider returns snapshots to render.
type SnapshotProvider func() ([]heartbeat.Snapshot, error)

// TrafficStats contains preformatted traffic counters for one state row.
type TrafficStats struct {
	Upload   string
	Download string
	Total    string
}

// SnapshotView contains all data needed to render a state table.
type SnapshotView struct {
	Snapshots []heartbeat.Snapshot
	Stats     map[string]TrafficStats
	ShowStats bool
}

// ViewProvider returns a complete state table view.
type ViewProvider func() (SnapshotView, error)

// Snapshot loads heartbeat state from disk and returns annotated entries.
func Snapshot(path string, ttl time.Duration) ([]heartbeat.Snapshot, error) {
	state, err := heartbeat.Load(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || heartbeat.IsCorrupt(err) {
			return nil, nil
		}
		return nil, err
	}
	return state.Snapshot(time.Now(), ttl), nil
}

// PrintWithSnapshots renders snapshots from a provider to stdout.
func PrintWithSnapshots(provider SnapshotProvider) error {
	return PrintWithView(func() (SnapshotView, error) {
		snapshots, err := provider()
		if err != nil {
			return SnapshotView{}, err
		}
		return SnapshotView{Snapshots: snapshots}, nil
	})
}

// PrintWithView renders a complete state view to stdout.
func PrintWithView(provider ViewProvider) error {
	view, err := provider()
	if err != nil {
		return err
	}
	RenderView(os.Stdout, view)
	return nil
}

// RenderView prints the heartbeat snapshot and optional stats.
func RenderView(w io.Writer, view SnapshotView) {
	if !view.ShowStats && len(view.Stats) == 0 {
		RenderTable(w, view.Snapshots)
		return
	}
	RenderTableWithStats(w, view.Snapshots, view.Stats)
}

// RenderTable prints the heartbeat snapshot as a tabular report.
func RenderTable(w io.Writer, snapshots []heartbeat.Snapshot) {
	renderTable(w, snapshots, nil, false)
}

// RenderTableWithStats prints the heartbeat snapshot with traffic counters.
func RenderTableWithStats(w io.Writer, snapshots []heartbeat.Snapshot, stats map[string]TrafficStats) {
	renderTable(w, snapshots, stats, true)
}

func renderTable(w io.Writer, snapshots []heartbeat.Snapshot, stats map[string]TrafficStats, withStats bool) {
	tw := tabwriter.NewWriter(w, 2, 2, 2, ' ', 0)
	header := "TAG\tHOST\tSTATUS\tLAST_RTT\tAVG_RTT\tLAST_UPDATE\tCLIENT_USER\tCLIENT_IP"
	if withStats {
		header += "\tUPLOAD\tDOWNLOAD\tTOTAL"
	}
	fmt.Fprintln(tw, header)
	if len(snapshots) == 0 {
		row := "-\t-\t-\t-\t-\t-\t-\t-"
		if withStats {
			row += "\t-\t-\t-"
		}
		fmt.Fprintln(tw, row)
	} else {
		for _, snap := range snapshots {
			status := "dead"
			if snap.Alive {
				status = "alive"
			}
			lastUpdate := "-"
			if !snap.Entry.LastSeen.IsZero() {
				lastUpdate = snap.Entry.LastSeen.UTC().Format(time.RFC3339)
			}
			user := safeClientUser(snap.Entry.User)
			fmt.Fprintf(tw, "%s\t%s\t%s\t%dms\t%.1fms\t%s\t%s\t%s",
				snap.Entry.Tag,
				snap.Entry.Host,
				status,
				snap.Entry.LastRTTMillis,
				snap.AvgRTTMillis,
				lastUpdate,
				user,
				snap.Entry.ClientIP,
			)
			if withStats {
				item, ok := stats[statsKey(user)]
				if !ok {
					item = TrafficStats{Upload: "-", Download: "-", Total: "-"}
				}
				fmt.Fprintf(tw, "\t%s\t%s\t%s", item.Upload, item.Download, item.Total)
			}
			fmt.Fprintln(tw)
		}
	}
	_ = tw.Flush()
}

func statsKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func safeClientUser(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

// Print renders the current state to stdout.
func Print(path string, ttl time.Duration) error {
	return PrintWithSnapshots(func() ([]heartbeat.Snapshot, error) {
		return Snapshot(path, ttl)
	})
}

// Watch repeatedly prints the state until the context is cancelled.
func Watch(ctx context.Context, path string, interval, ttl time.Duration) error {
	return WatchWithSnapshots(ctx, func() ([]heartbeat.Snapshot, error) {
		return Snapshot(path, ttl)
	}, interval)
}

// WatchWithSnapshots repeatedly prints snapshots until the context is cancelled.
func WatchWithSnapshots(ctx context.Context, provider SnapshotProvider, interval time.Duration) error {
	return WatchWithView(ctx, func() (SnapshotView, error) {
		snapshots, err := provider()
		if err != nil {
			return SnapshotView{}, err
		}
		return SnapshotView{Snapshots: snapshots}, nil
	}, interval)
}

// WatchWithView repeatedly prints a complete state view until the context is cancelled.
func WatchWithView(ctx context.Context, provider ViewProvider, interval time.Duration) error {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	if err := printViewWithClear(provider); err != nil {
		return err
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := printViewWithClear(provider); err != nil {
				return err
			}
		}
	}
}

func printSnapshotsWithClear(provider SnapshotProvider) error {
	return printViewWithClear(func() (SnapshotView, error) {
		snapshots, err := provider()
		if err != nil {
			return SnapshotView{}, err
		}
		return SnapshotView{Snapshots: snapshots}, nil
	})
}

func printViewWithClear(provider ViewProvider) error {
	clearTerminal(os.Stdout)
	return PrintWithView(provider)
}

func clearTerminal(w io.Writer) {
	if w == nil {
		return
	}
	_, _ = fmt.Fprint(w, "\033[H\033[2J")
}
