package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunStopsAfterMaxFailures(t *testing.T) {
	const maxRestarts = 3
	var runs int32

	err := Run(context.Background(), Options{
		Name:         "failing",
		MaxRestarts:  maxRestarts,
		RestartDelay: 10 * time.Millisecond,
	}, func(context.Context) error {
		atomic.AddInt32(&runs, 1)
		return fmt.Errorf("fail %d", runs)
	})
	if err == nil {
		t.Fatalf("expected error after max restarts, got nil")
	}
	if got := atomic.LoadInt32(&runs); int(got) != maxRestarts {
		t.Fatalf("expected %d runs, got %d", maxRestarts, got)
	}
}

func TestRunRestartsOnWatchEvent(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startCh := make(chan struct{}, 4)
	errCh := make(chan error, 1)

	go func() {
		errCh <- Run(ctx, Options{
			Name:         "watcher",
			WatchPaths:   []string{dir},
			RestartDelay: 10 * time.Millisecond,
		}, func(runCtx context.Context) error {
			startCh <- struct{}{}
			<-runCtx.Done()
			return runCtx.Err()
		})
	}()

	waitForStart(t, startCh)

	file := filepath.Join(dir, "config.json")
	if err := os.WriteFile(file, []byte(fmt.Sprintf("ts=%d", time.Now().UnixNano())), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	waitForStart(t, startCh)

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("service returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for service shutdown")
	}
}

func TestRunRestartsOnWatchFile(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	configPath := filepath.Join(dir, "xp2p-client.toml")
	if err := os.WriteFile(configPath, []byte("init = true\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	startCh := make(chan struct{}, 4)
	errCh := make(chan error, 1)

	go func() {
		errCh <- Run(ctx, Options{
			Name:          "file-watcher",
			WatchFiles:    []string{configPath},
			WatchDebounce: 10 * time.Millisecond,
			RestartDelay:  10 * time.Millisecond,
		}, func(runCtx context.Context) error {
			startCh <- struct{}{}
			<-runCtx.Done()
			return runCtx.Err()
		})
	}()

	waitForStart(t, startCh)

	if err := os.WriteFile(configPath, []byte(fmt.Sprintf("ts=%d\n", time.Now().UnixNano())), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	waitForStart(t, startCh)

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("service returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for service shutdown")
	}
}

func waitForStart(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for service start")
	}
}
