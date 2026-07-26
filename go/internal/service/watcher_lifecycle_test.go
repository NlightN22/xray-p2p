package service

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFileWatcherCloseStopsPendingDebounce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watched")
	if err := os.WriteFile(path, []byte("before"), 0o600); err != nil {
		t.Fatal(err)
	}
	watcher, err := newFileWatcher([]string{path}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	watcher.schedule(path)
	if err := watcher.Close(); err != nil {
		t.Fatal(err)
	}
	watcher.mu.Lock()
	defer watcher.mu.Unlock()
	if len(watcher.timers) != 0 {
		t.Fatalf("pending timers remain after close: %d", len(watcher.timers))
	}
}

func TestConfigWatcherStopIsIdempotentAndWaitsForCallback(t *testing.T) {
	dir := t.TempDir()
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	var startedOnce sync.Once
	stop, err := StartConfigWatcher(context.Background(), ConfigWatchOptions{
		Paths: []string{dir},
		OnChange: func(string) {
			calls.Add(1)
			startedOnce.Do(func() { close(started) })
			<-release
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "change"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("callback did not start")
	}
	var stops sync.WaitGroup
	stops.Add(2)
	for range 2 {
		go func() {
			defer stops.Done()
			stop()
		}()
	}
	select {
	case <-time.After(50 * time.Millisecond):
		if calls.Load() != 1 {
			t.Fatalf("unexpected callback count: %d", calls.Load())
		}
	}
	close(release)
	stops.Wait()
}
