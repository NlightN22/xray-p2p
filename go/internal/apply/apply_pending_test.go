package apply

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyPending_DoesNotDeleteLiveFilesWhenPendingDirEmpty(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	liveRoot := filepath.Join(root, "live")
	pendingRoot := filepath.Join(root, "pending")
	lkgRoot := filepath.Join(root, "lkg")

	liveConfigDir := filepath.Join(liveRoot, "config")
	pendingConfigDir := filepath.Join(pendingRoot, "config")

	if err := os.MkdirAll(liveConfigDir, 0o755); err != nil {
		t.Fatalf("mkdir live: %v", err)
	}
	if err := os.MkdirAll(lkgRoot, 0o755); err != nil {
		t.Fatalf("mkdir lkg: %v", err)
	}
	if err := os.WriteFile(filepath.Join(liveConfigDir, "inbounds.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write live json: %v", err)
	}

	pendingConfigFile := filepath.Join(pendingRoot, "xp2p-client.toml")
	if err := os.MkdirAll(filepath.Dir(pendingConfigFile), 0o755); err != nil {
		t.Fatalf("mkdir pending root: %v", err)
	}
	if err := os.WriteFile(pendingConfigFile, []byte("[client]\n"), 0o644); err != nil {
		t.Fatalf("write pending config: %v", err)
	}

	// pendingConfigDir intentionally does not exist / empty.
	_, applied, err := ApplyPending(PendingSet{
		LiveConfigFile:    filepath.Join(liveRoot, "xp2p-client.toml"),
		PendingConfigFile: pendingConfigFile,
		LiveConfigDir:     liveConfigDir,
		PendingConfigDir:  pendingConfigDir,
		LiveRoot:          liveRoot,
		LkgRoot:           lkgRoot,
		AuditPath:         "",
	})
	if err != nil {
		t.Fatalf("ApplyPending: %v", err)
	}
	if !applied {
		t.Fatalf("expected applied=true")
	}

	if _, err := os.Stat(filepath.Join(liveConfigDir, "inbounds.json")); err != nil {
		t.Fatalf("expected live json to remain: %v", err)
	}
}
