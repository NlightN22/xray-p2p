package apply

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWithRoleLockSerializesWriters(t *testing.T) {
	root := t.TempDir()
	var active atomic.Int32
	var overlap atomic.Bool
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := WithRoleLock(context.Background(), root, RoleClient, func() error {
				if active.Add(1) != 1 {
					overlap.Store(true)
				}
				time.Sleep(30 * time.Millisecond)
				active.Add(-1)
				return nil
			})
			if err != nil {
				t.Errorf("role lock: %v", err)
			}
		}()
	}
	wg.Wait()
	if overlap.Load() {
		t.Fatal("role writers overlapped")
	}
}

func TestSourceDigestChangesWithDesiredInputs(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "client.toml")
	extensionsDir := filepath.Join(root, "extensions")
	if err := os.MkdirAll(extensionsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("[client]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	first, err := SourceDigest(configPath, extensionsDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extensionsDir, "routing.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := SourceDigest(configPath, extensionsDir)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("digest did not change")
	}
}
