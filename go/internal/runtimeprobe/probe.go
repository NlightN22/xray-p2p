package runtimeprobe

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	EnvPath     = "XP2P_RUNTIME_METRICS_FILE"
	EnvInterval = "XP2P_RUNTIME_METRICS_INTERVAL"
)

// Start publishes opt-in process runtime metrics until ctx is cancelled.
func Start(ctx context.Context) {
	path := strings.TrimSpace(os.Getenv(EnvPath))
	if path == "" {
		return
	}
	interval := time.Second
	if value := strings.TrimSpace(os.Getenv(EnvInterval)); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil && parsed > 0 {
			interval = parsed
		}
	}
	write(path)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				write(path)
			}
		}
	}()
}

func write(path string) {
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	content := fmt.Sprintf(
		"pid=%d\ngo_heap_alloc=%d\ngo_heap_sys=%d\ngo_goroutines=%d\nsample_unix_nano=%d\n",
		os.Getpid(),
		memory.HeapAlloc,
		memory.HeapSys,
		runtime.NumGoroutine(),
		time.Now().UnixNano(),
	)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	temporary := path + ".tmp-" + strconv.Itoa(os.Getpid())
	if err := os.WriteFile(temporary, []byte(content), 0o600); err != nil {
		return
	}
	_ = os.Rename(temporary, path)
}
