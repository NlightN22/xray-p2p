//go:build windows

package client

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestInstallCreatesConfigAndState(t *testing.T) {
	dir := t.TempDir()

	// Prepare stub xray binary so the installer can proceed.
	binDir := filepath.Join(dir, layout.BinDirName)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	xrayPath := filepath.Join(binDir, "xray.exe")
	if err := os.WriteFile(xrayPath, []byte("stub"), 0o755); err != nil {
		t.Fatalf("write stub xray: %v", err)
	}

	opts := InstallOptions{
		InstallDir:    dir,
		ConfigDir:     DefaultClientConfigDir,
		ServerAddress: "edge.example.com",
		ServerPort:    "62022",
		User:          "user@example.com",
		Password:      "secret",
		ServerName:    "edge.example.com",
	}
	if err := Install(context.Background(), opts); err != nil {
		t.Fatalf("Install returned error: %v", err)
	}

	configDir := filepath.Join(dir, DefaultClientConfigDir)
	for _, name := range []string{"inbounds.json", "logs.json", "outbounds.json", "routing.json"} {
		if _, err := os.Stat(filepath.Join(configDir, name)); err != nil {
			t.Fatalf("expected %s to be created: %v", name, err)
		}
	}

	statePath := filepath.Join(dir, layout.ClientStateFileName)
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state file: %v", err)
	}
	var state clientInstallState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("decode client state: %v", err)
	}
	if len(state.Endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(state.Endpoints))
	}
	ep := state.Endpoints[0]
	if ep.Hostname != "edge.example.com" || ep.Port != 62022 {
		t.Fatalf("unexpected endpoint record: %+v", ep)
	}
	if ep.User != "user@example.com" || ep.Password != "secret" {
		t.Fatalf("unexpected credentials: %+v", ep)
	}
}

func TestInstallFailsWhenXrayMissing(t *testing.T) {
	dir := t.TempDir()
	opts := InstallOptions{
		InstallDir:    dir,
		ConfigDir:     DefaultClientConfigDir,
		ServerAddress: "edge.example.com",
		ServerPort:    "62022",
		User:          "user@example.com",
		Password:      "secret",
	}
	err := Install(context.Background(), opts)
	if err == nil {
		t.Fatalf("expected error when xray binary is absent")
	}
	if !strings.Contains(err.Error(), "xray binary missing") {
		t.Fatalf("unexpected error: %v", err)
	}
}
