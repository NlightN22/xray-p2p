package config

import (
	"path/filepath"
	"testing"

	"github.com/pelletier/go-toml"
)

func TestUpdateTunEnabledWritesValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "xp2p.toml")
	updated, err := UpdateTunEnabled(path, "client", true)
	if err != nil {
		t.Fatalf("UpdateTunEnabled failed: %v", err)
	}
	if updated != path {
		t.Fatalf("unexpected path %s", updated)
	}
	tree, err := toml.LoadFile(path)
	if err != nil {
		t.Fatalf("load toml: %v", err)
	}
	value := tree.GetPath([]string{"client", "tun_enabled"})
	if value != true {
		t.Fatalf("unexpected tun_enabled: %v", value)
	}
}
