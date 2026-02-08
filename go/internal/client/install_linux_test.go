//go:build linux

package client

import (
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestNormalizeInstallOptionsLinux(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", tempDir)

	opts := InstallOptions{
		InstallDir:    tempDir,
		ServerAddress: "vpn.example",
		ServerPort:    "9443",
		User:          "user@example.com",
		Password:      "secret",
		ServerName:    "vpn.example",
		AllowInsecure: true,
	}

	state, err := normalizeInstallOptions(opts)
	if err != nil {
		t.Fatalf("normalizeInstallOptions: %v", err)
	}

	expectedConfig := filepath.Join(tempDir, layout.ClientConfigDir)
	if state.configDir != expectedConfig {
		t.Fatalf("configDir mismatch: got %s want %s", state.configDir, expectedConfig)
	}

	expectedLogDir := filepath.Join(layout.UnixLogRoot, "client")
	if state.logsDir != expectedLogDir {
		t.Fatalf("logsDir mismatch: got %s want %s", state.logsDir, expectedLogDir)
	}

	expectedState := filepath.Clean(config.ConfigPath(layout.ClientAppliedStateFileName))
	if state.stateFile != expectedState {
		t.Fatalf("stateFile mismatch: got %s want %s", state.stateFile, expectedState)
	}
}
