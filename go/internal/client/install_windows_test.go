//go:build windows

package client

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestInstallCreatesConfigAndState(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)

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

	configPath := filepath.Clean(config.ConfigPath(layout.ClientConfigFileName))
	state, err := loadClientInstallState(configPath)
	if err != nil {
		t.Fatalf("read config state: %v", err)
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

	appliedPath := filepath.Clean(config.ConfigPath(layout.ClientAppliedStateFileName))
	applied, err := loadClientAppliedState(appliedPath)
	if err != nil {
		t.Fatalf("read applied state: %v", err)
	}
	if applied.Mode != "tun" {
		t.Fatalf("unexpected applied mode: %s", applied.Mode)
	}
	if len(applied.Config.Endpoints) != 1 {
		t.Fatalf("expected 1 applied endpoint, got %d", len(applied.Config.Endpoints))
	}
}

func TestInstallFailsWhenXrayMissing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
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

func TestInstallPreservesExistingInboundsAndLogs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)

	binDir := filepath.Join(dir, layout.BinDirName)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	xrayPath := filepath.Join(binDir, "xray.exe")
	if err := os.WriteFile(xrayPath, []byte("stub"), 0o755); err != nil {
		t.Fatalf("write stub xray: %v", err)
	}

	configDir := filepath.Join(dir, DefaultClientConfigDir)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	inboundsPath := filepath.Join(configDir, "inbounds.json")
	logsPath := filepath.Join(configDir, "logs.json")
	inboundsContents := []byte("{\"custom\": \"inbounds\"}\n")
	logsContents := []byte("{\"custom\": \"logs\"}\n")
	if err := os.WriteFile(inboundsPath, inboundsContents, 0o644); err != nil {
		t.Fatalf("write inbounds: %v", err)
	}
	if err := os.WriteFile(logsPath, logsContents, 0o644); err != nil {
		t.Fatalf("write logs: %v", err)
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

	gotInbounds, err := os.ReadFile(inboundsPath)
	if err != nil {
		t.Fatalf("read inbounds: %v", err)
	}
	if string(gotInbounds) != string(inboundsContents) {
		t.Fatalf("inbounds changed: got %q want %q", gotInbounds, inboundsContents)
	}
	gotLogs, err := os.ReadFile(logsPath)
	if err != nil {
		t.Fatalf("read logs: %v", err)
	}
	if string(gotLogs) != string(logsContents) {
		t.Fatalf("logs changed: got %q want %q", gotLogs, logsContents)
	}
}
