package client

import (
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
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
	if base.configFile != filepath.Clean(config.PendingConfigPath(layout.ClientConfigFileName)) {
		t.Fatalf("configFile = %s", base.configFile)
	}
	if base.appliedStateFile != filepath.Clean(config.ConfigPath(layout.ClientAppliedStateFileName)) {
		t.Fatalf("appliedStateFile = %s", base.appliedStateFile)
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
	if base.installOpts.TunAddr != "198.18.0.1/30" {
		t.Fatalf("expected tun addr 198.18.0.1/30, got %s", base.installOpts.TunAddr)
	}
	if base.installOpts.TunMode != "split" {
		t.Fatalf("expected tun mode split, got %s", base.installOpts.TunMode)
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

func TestBuildClientInstallBaseAppliesExplicitProfile(t *testing.T) {
	installDir := t.TempDir()
	configDir := filepath.Join(installDir, "config-client")

	base, err := buildClientInstallBase(installDir, configDir, InstallOptions{
		InstallDir:    installDir,
		ConfigDir:     "config-client",
		ServerAddress: "edge.example.com",
		User:          "user@example.com",
		Password:      "550e8400-e29b-41d4-a716-446655440000",
		Profile:       "vless-tls-vision",
	})
	if err != nil {
		t.Fatalf("buildClientInstallBase error: %v", err)
	}
	if base.installOpts.Profile != "vless-tls-vision" || base.installOpts.Protocol != "vless" || base.installOpts.Flow != "xtls-rprx-vision" {
		t.Fatalf("profile defaults were not applied: %+v", base.installOpts)
	}
}

func TestBuildClientInstallBaseRejectsInvalidProfile(t *testing.T) {
	installDir := t.TempDir()
	configDir := filepath.Join(installDir, "config-client")

	if _, err := buildClientInstallBase(installDir, configDir, InstallOptions{
		InstallDir:    installDir,
		ConfigDir:     "config-client",
		ServerAddress: "edge.example.com",
		User:          "user@example.com",
		Password:      "secret",
		Profile:       "unknown",
	}); err == nil {
		t.Fatalf("expected error for unknown profile")
	}
}
