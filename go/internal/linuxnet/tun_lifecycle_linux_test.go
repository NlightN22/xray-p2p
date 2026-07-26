//go:build linux

package linuxnet

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEnsureTunAddressCancelsRunningIPCommand(t *testing.T) {
	started := installBlockingIP(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- EnsureTunAddressContext(ctx, "xp2p-test", "198.18.0.1/30", 1500)
	}()
	waitForMarker(t, started)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("EnsureTunAddressContext error = %v, want context canceled", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("running ip command outlived cancellation")
	}
}

func TestEnsureRouteCancelsRunningIPCommand(t *testing.T) {
	started := installBlockingIP(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- EnsureRouteContext(ctx, "xp2p-test", "10.20.0.0/16")
	}()
	waitForMarker(t, started)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("EnsureRouteContext error = %v, want context canceled", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("running route command outlived cancellation")
	}
}

func installBlockingIP(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	started := filepath.Join(dir, "started")
	script := filepath.Join(dir, "ip")
	data := []byte("#!/bin/sh\n: > \"$XP2P_IP_STARTED\"\nsleep 30\n")
	if err := os.WriteFile(script, data, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XP2P_IP_STARTED", started)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return started
}

func waitForMarker(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("blocking ip command did not start")
}
