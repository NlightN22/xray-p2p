//go:build windows || linux

package server

import (
	"context"
	"os"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/xraylive"
)

func TestSetProfileStagesDesiredWhenServiceIsStopped(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	if err := os.WriteFile(config.ConfigPath(layout.ServerConfigFileName), []byte("[server]\n"), 0o644); err != nil {
		t.Fatalf("write server config: %v", err)
	}

	result, err := SetProfile(context.Background(), SetProfileOptions{Profile: "trojan-tls"})
	if err != nil {
		t.Fatalf("SetProfile failed: %v", err)
	}
	if result.Profile != "trojan-tls" || result.Apply != xraylive.RuntimeApplyStaged {
		t.Fatalf("unexpected result: %+v", result)
	}
	loaded, err := config.Load(config.Options{Path: config.ConfigPath(layout.ServerConfigFileName)})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if loaded.Server.Profile != "trojan-tls" {
		t.Fatalf("profile was not persisted: %q", loaded.Server.Profile)
	}
}

func TestSetProfileRejectsUnknownProfile(t *testing.T) {
	if _, err := SetProfile(context.Background(), SetProfileOptions{Profile: "unknown"}); err == nil {
		t.Fatalf("expected unknown profile error")
	}
}
