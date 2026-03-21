package config

import (
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/pelletier/go-toml"
)

func TestUpdateTunEnabledWritesValue(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	path := filepath.Clean(ConfigPath(layout.ClientConfigFileName))
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

func TestUpdateTunModeWritesValue(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	path := filepath.Clean(ConfigPath(layout.ClientConfigFileName))
	updated, err := UpdateTunMode(path, "client", "full")
	if err != nil {
		t.Fatalf("UpdateTunMode failed: %v", err)
	}
	if updated != path {
		t.Fatalf("unexpected path %s", updated)
	}
	tree, err := toml.LoadFile(path)
	if err != nil {
		t.Fatalf("load toml: %v", err)
	}
	value := tree.GetPath([]string{"client", "tun_mode"})
	if value != "full" {
		t.Fatalf("unexpected tun_mode: %v", value)
	}
}
