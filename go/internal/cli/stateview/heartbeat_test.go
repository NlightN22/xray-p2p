package stateview

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/heartbeat"
)

func TestSnapshotLoadsState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "heartbeat.json")
	state := heartbeat.State{
		Entries: map[string]heartbeat.Entry{
			"alpha": {
				Tag:            "alpha",
				Host:           "edge-a",
				LastSeen:       time.Now().Add(-time.Minute),
				LastRTTMillis:  25,
				Samples:        1,
				TotalRTTMillis: 25,
			},
		},
	}
	if err := heartbeat.Save(path, state); err != nil {
		t.Fatalf("save heartbeat state: %v", err)
	}

	snapshots, err := Snapshot(path, 2*time.Minute)
	if err != nil {
		t.Fatalf("Snapshot returned error: %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("Snapshot len = %d, want 1", len(snapshots))
	}
	if !snapshots[0].Alive {
		t.Fatalf("expected snapshot to be alive")
	}

	missing, err := Snapshot(filepath.Join(dir, "missing.json"), time.Minute)
	if err != nil {
		t.Fatalf("Snapshot on missing path returned error: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected nil snapshot for missing file")
	}
}

func TestRenderTableOutputsRows(t *testing.T) {
	buf := new(bytes.Buffer)
	RenderTable(buf, nil)
	first := buf.String()
	if !strings.Contains(first, "TAG") || !strings.Contains(first, "-    -") {
		t.Fatalf("expected placeholder row, got %q", first)
	}

	buf.Reset()
	snapshots := []heartbeat.Snapshot{
		{
			Entry: heartbeat.Entry{
				Tag:           "alpha",
				Host:          "edge-a",
				LastRTTMillis: 20,
				LastSeen:      time.Unix(1700000000, 0),
				User:          "user@example.com",
				ClientIP:      "203.0.113.5",
			},
			AvgRTTMillis: 22.5,
			Alive:        true,
		},
	}
	RenderTable(buf, snapshots)
	out := buf.String()
	if !strings.Contains(out, "alpha") || !strings.Contains(out, "edge-a") || !strings.Contains(out, "alive") {
		t.Fatalf("unexpected render output: %q", out)
	}
	if !strings.Contains(out, "user@example.com") {
		t.Fatalf("expected client user in output: %q", out)
	}
}

func TestSafeClientUser(t *testing.T) {
	if safeClientUser("  ") != "-" {
		t.Fatalf("safeClientUser should return '-' for blanks")
	}
	if safeClientUser("alice") != "alice" {
		t.Fatalf("safeClientUser returned unexpected value")
	}
}

func TestPrintRendersSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	state := heartbeat.State{
		Entries: map[string]heartbeat.Entry{
			"alpha": {Tag: "alpha", Host: "edge-a"},
		},
	}
	if err := heartbeat.Save(path, state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	output := captureOutput(t, func() {
		if err := Print(path, time.Minute); err != nil {
			t.Fatalf("Print returned error: %v", err)
		}
	})
	if !strings.Contains(output, "edge-a") {
		t.Fatalf("expected host in output: %q", output)
	}
}

func TestWatchStopsOnCancel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := heartbeat.Save(path, heartbeat.State{}); err != nil {
		t.Fatalf("save state: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	go func() {
		done <- Watch(ctx, path, 10*time.Millisecond, time.Second)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()
	w.Close()
	os.Stdout = origStdout
	_, _ = io.Copy(io.Discard, r)

	err := <-done
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Watch returned %v, want context.Canceled", err)
	}
}

func captureOutput(t *testing.T, fn func()) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	fn()
	if err := w.Close(); err != nil {
		t.Fatalf("close stdout: %v", err)
	}
	os.Stdout = oldStdout
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return buf.String()
}
