package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func TestRunHandlesWatchFileWithoutRestart(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	configPath := filepath.Join(dir, "apply.request")
	if err := os.WriteFile(configPath, []byte("init\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	startCh := make(chan struct{}, 4)
	handledCh := make(chan struct{}, 1)
	errCh := make(chan error, 1)

	go func() {
		errCh <- Run(ctx, Options{
			Name:          "file-handler",
			WatchFiles:    []string{configPath},
			WatchDebounce: 10 * time.Millisecond,
			RestartDelay:  10 * time.Millisecond,
			OnWatchFileChange: func(context.Context, string) (WatchFileAction, error) {
				handledCh <- struct{}{}
				return WatchFileHandled, nil
			},
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

	select {
	case <-handledCh:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for watched file handler")
	}
	waitForNoStart(t, startCh, 300*time.Millisecond)

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

func TestRunWatchLimiterSkipsAfterThreshold(t *testing.T) {
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
			Name:               "watch-limiter",
			WatchFiles:         []string{configPath},
			WatchDebounce:      10 * time.Millisecond,
			RestartDelay:       10 * time.Millisecond,
			MaxWatchRestarts:   1,
			WatchRestartWindow: 2 * time.Second,
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

	if err := os.WriteFile(configPath, []byte(fmt.Sprintf("ts=%d\n", time.Now().UnixNano()+1)), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	waitForNoStart(t, startCh, 600*time.Millisecond)

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

func TestRunReportsChildShutdownTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	release := make(chan struct{})
	go func() {
		<-started
		cancel()
	}()
	err := Run(ctx, Options{
		Name:            "stuck",
		ShutdownTimeout: 20 * time.Millisecond,
	}, func(context.Context) error {
		close(started)
		<-release
		return nil
	})
	close(release)
	if err == nil || !strings.Contains(err.Error(), "child shutdown timed out") {
		t.Fatalf("expected child shutdown timeout, got %v", err)
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

func waitForNoStart(t *testing.T, ch <-chan struct{}, d time.Duration) {
	t.Helper()
	select {
	case <-ch:
		t.Fatal("unexpected service restart")
	case <-time.After(d):
	}
}
