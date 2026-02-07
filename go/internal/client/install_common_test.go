package client

import (
	"path/filepath"
	"testing"
)

func TestBuildClientInstallBaseDefaults(t *testing.T) {
	installDir := t.TempDir()
	configDir := filepath.Join(installDir, "config-client")

	base, err := buildClientInstallBase(installDir, configDir, InstallOptions{
		InstallDir:    installDir,
		ConfigDir:     "config-client",
		ServerAddress: "edge.example.com",
		User:          "user@example.com",
		Password:      "secret",
	})
	if err != nil {
		t.Fatalf("buildClientInstallBase error: %v", err)
	}
	if base.portStr != "8443" || base.portVal != 8443 {
		t.Fatalf("port = %s (%d)", base.portStr, base.portVal)
	}
	if base.serverName != "edge.example.com" {
		t.Fatalf("serverName = %s", base.serverName)
	}
	if base.stateFile != filepath.Join(installDir, "install-state-client.json") {
		t.Fatalf("stateFile = %s", base.stateFile)
	}
	if !base.installOpts.TunEnabled {
		t.Fatalf("expected tun enabled by default")
	}
	if base.installOpts.TunName != "xp2pc" {
		t.Fatalf("expected tun name xp2pc, got %s", base.installOpts.TunName)
	}
	if base.installOpts.TunMTU != 1500 {
		t.Fatalf("expected tun MTU 1500, got %d", base.installOpts.TunMTU)
	}
}

func TestBuildClientInstallBaseRequiresInputs(t *testing.T) {
	installDir := t.TempDir()
	configDir := filepath.Join(installDir, "config-client")

	_, err := buildClientInstallBase(installDir, configDir, InstallOptions{
		InstallDir: installDir,
		ConfigDir:  "config-client",
		User:       "user@example.com",
		Password:   "secret",
	})
	if err == nil {
		t.Fatalf("expected error for missing server address")
	}
}
