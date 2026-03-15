//go:build linux || windows

package client

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestApplyModeDoesNotWriteAppliedState(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)

	installDir := filepath.Join(dir, "xp2p")
	configDir := filepath.Join(installDir, DefaultClientConfigDir)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}

	opts := ModeOptions{
		InstallDir: installDir,
		ConfigDir:  DefaultClientConfigDir,
		TunEnabled: true,
		TunName:    "xp2pc",
		TunMTU:     1500,
		TunAddr:    "198.18.0.1/30",
	}
	if err := ApplyMode(opts); err == nil {
		t.Fatalf("expected error when client config is missing")
	}

	appliedPath := filepath.Clean(config.ConfigPath(layout.ClientAppliedStateFileName))
	if _, err := os.Stat(appliedPath); err == nil {
		t.Fatalf("expected applied state to be untouched, got %s", appliedPath)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat applied state: %v", err)
	}
}
