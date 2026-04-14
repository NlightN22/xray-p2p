package apply

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotPendingFromDesired_UsesDesiredConfigWhenPresent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	desiredConfig := filepath.Join(root, "xp2p-client.toml")
	liveConfig := filepath.Join(root, ".state", "live", "xp2p-client.toml")
	pendingConfig := filepath.Join(root, ".state", "pending", "xp2p-client.toml")

	desiredDir := filepath.Join(root, "config-client")
	liveDir := filepath.Join(root, ".state", "live", "config-client")
	pendingDir := filepath.Join(root, ".state", "pending", "config-client")

	if err := os.MkdirAll(filepath.Dir(liveConfig), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(liveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(desiredDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := os.WriteFile(liveConfig, []byte("live\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(desiredConfig, []byte("desired\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(liveDir, "a.json"), []byte("live-a\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(desiredDir, "a.json"), []byte("desired-a\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(desiredDir, "b.json"), []byte("desired-b\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	wrote, err := SnapshotPendingFromDesired(PendingSnapshotOptions{
		DesiredConfigFile: desiredConfig,
		DesiredConfigDir:  desiredDir,
		LiveConfigFile:    liveConfig,
		LiveConfigDir:     liveDir,
		PendingConfigFile: pendingConfig,
		PendingConfigDir:  pendingDir,
	})
	if err != nil {
		t.Fatalf("SnapshotPendingFromDesired: %v", err)
	}
	if !wrote {
		t.Fatalf("expected wrote=true")
	}

	gotConfig, err := os.ReadFile(pendingConfig)
	if err != nil {
		t.Fatalf("read pending config: %v", err)
	}
	if string(gotConfig) != "desired\n" {
		t.Fatalf("pending config mismatch: %q", string(gotConfig))
	}
	gotA, err := os.ReadFile(filepath.Join(pendingDir, "a.json"))
	if err != nil {
		t.Fatalf("read pending a.json: %v", err)
	}
	if string(gotA) != "desired-a\n" {
		t.Fatalf("pending a.json mismatch: %q", string(gotA))
	}
	gotB, err := os.ReadFile(filepath.Join(pendingDir, "b.json"))
	if err != nil {
		t.Fatalf("read pending b.json: %v", err)
	}
	if string(gotB) != "desired-b\n" {
		t.Fatalf("pending b.json mismatch: %q", string(gotB))
	}
}

func TestSnapshotPendingFromDesired_UsesLiveWhenDesiredMissing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	desiredConfig := filepath.Join(root, "xp2p-client.toml")
	liveConfig := filepath.Join(root, ".state", "live", "xp2p-client.toml")
	pendingConfig := filepath.Join(root, ".state", "pending", "xp2p-client.toml")

	desiredDir := filepath.Join(root, "config-client")
	liveDir := filepath.Join(root, ".state", "live", "config-client")
	pendingDir := filepath.Join(root, ".state", "pending", "config-client")

	if err := os.MkdirAll(filepath.Dir(liveConfig), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(liveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := os.WriteFile(liveConfig, []byte("live\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(liveDir, "a.json"), []byte("live-a\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	wrote, err := SnapshotPendingFromDesired(PendingSnapshotOptions{
		DesiredConfigFile: desiredConfig,
		DesiredConfigDir:  desiredDir,
		LiveConfigFile:    liveConfig,
		LiveConfigDir:     liveDir,
		PendingConfigFile: pendingConfig,
		PendingConfigDir:  pendingDir,
	})
	if err != nil {
		t.Fatalf("SnapshotPendingFromDesired: %v", err)
	}
	if !wrote {
		t.Fatalf("expected wrote=true")
	}

	gotConfig, err := os.ReadFile(pendingConfig)
	if err != nil {
		t.Fatalf("read pending config: %v", err)
	}
	if string(gotConfig) != "live\n" {
		t.Fatalf("pending config mismatch: %q", string(gotConfig))
	}
	gotA, err := os.ReadFile(filepath.Join(pendingDir, "a.json"))
	if err != nil {
		t.Fatalf("read pending a.json: %v", err)
	}
	if string(gotA) != "live-a\n" {
		t.Fatalf("pending a.json mismatch: %q", string(gotA))
	}
}
