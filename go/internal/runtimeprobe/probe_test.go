package runtimeprobe

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestStartWritesProcessMetrics(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.metrics")
	t.Setenv(EnvPath, path)
	t.Setenv(EnvInterval, "10ms")
	ctx, cancel := context.WithCancel(context.Background())
	Start(ctx)
	defer cancel()

	var content string
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			content = string(data)
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	for _, expected := range []string{
		"pid=" + strconv.Itoa(os.Getpid()),
		"go_heap_alloc=",
		"go_heap_sys=",
		"go_goroutines=",
		"sample_unix_nano=",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("metrics %q do not contain %q", content, expected)
		}
	}
}
