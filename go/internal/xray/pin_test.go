package xray

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPinnedVersionMatchesFile(t *testing.T) {
	version, err := PinnedVersion()
	if err != nil {
		t.Fatalf("PinnedVersion error: %v", err)
	}

	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repoRoot error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "go", "internal", "xray", "pinned.json"))
	if err != nil {
		t.Fatalf("read pinned.json: %v", err)
	}
	var cfg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("decode pinned.json: %v", err)
	}
	if version != cfg.Version {
		t.Fatalf("pinned version mismatch: got %q, want %q", version, cfg.Version)
	}
}

func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
