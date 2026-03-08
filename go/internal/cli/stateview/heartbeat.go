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
	snapshots, err := provider()
	if err != nil {
		return err
	}
	RenderTable(os.Stdout, snapshots)
	return nil
}

// RenderTable prints the heartbeat snapshot as a tabular report.
func RenderTable(w io.Writer, snapshots []heartbeat.Snapshot) {
	tw := tabwriter.NewWriter(w, 2, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "TAG\tHOST\tSTATUS\tLAST_RTT\tAVG_RTT\tLAST_UPDATE\tCLIENT_USER\tCLIENT_IP")
	if len(snapshots) == 0 {
		fmt.Fprintln(tw, "-\t-\t-\t-\t-\t-\t-\t-")
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
			fmt.Fprintf(tw, "%s\t%s\t%s\t%dms\t%.1fms\t%s\t%s\t%s\n",
				snap.Entry.Tag,
				snap.Entry.Host,
				status,
				snap.Entry.LastRTTMillis,
				snap.AvgRTTMillis,
				lastUpdate,
				safeClientUser(snap.Entry.User),
				snap.Entry.ClientIP,
			)
		}
	}
	_ = tw.Flush()
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
	if interval <= 0 {
		interval = 2 * time.Second
	}
	if err := printSnapshotsWithClear(provider); err != nil {
		return err
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := printSnapshotsWithClear(provider); err != nil {
				return err
			}
		}
	}
}

func printSnapshotsWithClear(provider SnapshotProvider) error {
	clearTerminal(os.Stdout)
	return PrintWithSnapshots(provider)
}

func clearTerminal(w io.Writer) {
	if w == nil {
		return
	}
	_, _ = fmt.Fprint(w, "\033[H\033[2J")
}
