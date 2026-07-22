//go:build windows

package client

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/extensions"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestInstallCreatesConfigAndState(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(dir, "logs"))

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
		ServerAddress: "localhost",
		ServerPort:    "58443",
		User:          "user@example.com",
		Password:      "secret",
		ServerName:    "localhost",
		HeartbeatMode: "auto",
	}
	if err := Install(context.Background(), opts); err != nil {
		t.Fatalf("Install returned error: %v", err)
	}

	for _, name := range []string{
		extensions.RoutingAfterSystemFile,
		extensions.RoutingAfterManagedFile,
		extensions.InboundsAppendFile,
		extensions.OutboundsAppendFile,
	} {
		if _, err := os.Stat(filepath.Join(dir, layout.ClientConfigDir, name)); err != nil {
			t.Fatalf("expected extension template %s: %v", name, err)
		}
	}

	if _, err := os.Stat(config.ApplyRequestPath()); err != nil {
		t.Fatalf("expected apply request: %v", err)
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
	if ep.Hostname != "localhost" || ep.Port != 58443 {
		t.Fatalf("unexpected endpoint record: %+v", ep)
	}
	if ep.User != "user@example.com" || ep.Password != "secret" {
		t.Fatalf("unexpected credentials: %+v", ep)
	}
	if ep.HeartbeatMode != "auto" {
		t.Fatalf("unexpected heartbeat mode: %q", ep.HeartbeatMode)
	}
}

func TestInstallFailsWhenXrayMissing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(dir, "logs"))
	opts := InstallOptions{
		InstallDir:    dir,
		ConfigDir:     DefaultClientConfigDir,
		ServerAddress: "edge.example.com",
		ServerPort:    "58443",
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

func TestInstallRewritesInboundsAndLogs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(dir, "logs"))

	binDir := filepath.Join(dir, layout.BinDirName)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	xrayPath := filepath.Join(binDir, "xray.exe")
	if err := os.WriteFile(xrayPath, []byte("stub"), 0o755); err != nil {
		t.Fatalf("write stub xray: %v", err)
	}

	extensionsDir := filepath.Join(dir, layout.ClientConfigDir)
	if err := os.MkdirAll(extensionsDir, 0o755); err != nil {
		t.Fatalf("mkdir extensions: %v", err)
	}
	customPath := filepath.Join(extensionsDir, extensions.InboundsAppendFile)
	customContents := []byte("{\"inbounds\":[{\"tag\":\"custom-in\"}]}\n")
	if err := os.WriteFile(customPath, customContents, 0o644); err != nil {
		t.Fatalf("write custom snippet: %v", err)
	}

	opts := InstallOptions{
		InstallDir:    dir,
		ConfigDir:     DefaultClientConfigDir,
		ServerAddress: "localhost",
		ServerPort:    "58443",
		User:          "user@example.com",
		Password:      "secret",
		ServerName:    "localhost",
	}
	if err := Install(context.Background(), opts); err != nil {
		t.Fatalf("Install returned error: %v", err)
	}

	got, err := os.ReadFile(customPath)
	if err != nil {
		t.Fatalf("read custom snippet: %v", err)
	}
	if string(got) != string(customContents) {
		t.Fatalf("expected extension snippet to remain untouched")
	}
	for _, name := range []string{
		extensions.RoutingAfterSystemFile,
		extensions.RoutingAfterManagedFile,
		extensions.OutboundsAppendFile,
	} {
		if _, err := os.Stat(filepath.Join(extensionsDir, name)); err != nil {
			t.Fatalf("expected template %s: %v", name, err)
		}
	}
}
