//go:build linux

package openwrt

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunCommandContextCancelsActiveOpenWrtCommand(t *testing.T) {
	dir := t.TempDir()
	command := filepath.Join(dir, "blocked-command")
	if err := os.WriteFile(command, []byte("#!/bin/sh\ntouch \"$1\"\nwhile :; do sleep 1; done\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "started")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runCommandContext(ctx, command, marker) }()

	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("OpenWrt command did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("command error = %v, want context cancellation", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("active OpenWrt command outlived cancellation")
	}
}
