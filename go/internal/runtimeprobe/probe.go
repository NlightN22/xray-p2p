package runtimeprobe

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	EnvPath     = "XP2P_RUNTIME_METRICS_FILE"
	EnvInterval = "XP2P_RUNTIME_METRICS_INTERVAL"
)

var (
	providersMu sync.RWMutex
	providers   = map[string]func() map[string]int64{}
	providerID  atomic.Uint64
)

// Register adds metrics to the opt-in runtime snapshot.
func Register(name string, provider func() map[string]int64) func() {
	key := fmt.Sprintf("%s-%d", name, providerID.Add(1))
	providersMu.Lock()
	providers[key] = provider
	providersMu.Unlock()
	return func() {
		providersMu.Lock()
		delete(providers, key)
		providersMu.Unlock()
	}
}

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
	providersMu.RLock()
	extra := map[string]int64{}
	for _, provider := range providers {
		for key, value := range provider() {
			extra[key] += value
		}
	}
	providersMu.RUnlock()
	for key, value := range extra {
		content += fmt.Sprintf("%s=%d\n", key, value)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	temporary := path + ".tmp-" + strconv.Itoa(os.Getpid())
	if err := os.WriteFile(temporary, []byte(content), 0o600); err != nil {
		return
	}
	_ = os.Rename(temporary, path)
}
