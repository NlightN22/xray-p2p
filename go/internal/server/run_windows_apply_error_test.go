//go:build windows

package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestRun_RollbackAndApplyErrorOnEarlyFailureAfterPendingApply(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))

	installDir := filepath.Join(root, "install")
	xrayPath := filepath.Join(installDir, layout.BinDirName, "xray.exe")
	if err := os.MkdirAll(filepath.Dir(xrayPath), 0o755); err != nil {
		t.Fatalf("mkdir xray: %v", err)
	}
	if err := os.WriteFile(xrayPath, []byte("xray\n"), 0o644); err != nil {
		t.Fatalf("write xray: %v", err)
	}

	if err := os.MkdirAll(config.StateRoot(), 0o755); err != nil {
		t.Fatalf("mkdir state: %v", err)
	}

	configDirName := DefaultServerConfigDir
	desiredConfigDir := filepath.Join(config.ConfigRoot(), configDirName)
	liveConfigDir, err := config.LiveConfigDir(desiredConfigDir)
	if err != nil {
		t.Fatalf("LiveConfigDir: %v", err)
	}

	baselineXray := []byte("{\"inbounds\":[]}\n")
	baselineRuntime := []byte("{\"role\":\"server\"}\n")
	if err := os.MkdirAll(liveConfigDir, 0o755); err != nil {
		t.Fatalf("mkdir live dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(liveConfigDir, layout.XrayConfigFileName), baselineXray, 0o644); err != nil {
		t.Fatalf("write baseline xray: %v", err)
	}
	if err := os.WriteFile(filepath.Join(liveConfigDir, layout.RuntimeMetaFileName), baselineRuntime, 0o644); err != nil {
		t.Fatalf("write baseline runtime: %v", err)
	}

	appliedStatePath := config.ConfigPath(layout.ServerAppliedStateFileName)
	if err := os.MkdirAll(filepath.Dir(appliedStatePath), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.WriteFile(appliedStatePath, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write applied state: %v", err)
	}

	pendingConfig := config.PendingConfigPath(layout.ServerConfigFileName)
	if err := os.WriteFile(pendingConfig, []byte("[server]\n"), 0o644); err != nil {
		t.Fatalf("write desired config: %v", err)
	}

	req, err := apply.NewRequest(apply.RoleServer)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Timestamp = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := apply.WriteRequest(config.ApplyRequestPath(), req, ""); err != nil {
		t.Fatalf("WriteRequest: %v", err)
	}

	runErr := Run(context.Background(), RunOptions{
		InstallDir: installDir,
		ConfigDir:  configDirName,
	})
	if runErr == nil {
		t.Fatalf("expected Run to fail")
	}

	marker, exists, err := apply.ReadError(config.ApplyErrorPath())
	if err != nil {
		t.Fatalf("ReadError: %v", err)
	}
	if !exists {
		t.Fatalf("expected apply.error to exist")
	}
	if marker.RequestID != req.ID {
		t.Fatalf("request_id mismatch: %q != %q", marker.RequestID, req.ID)
	}
	if marker.Role != apply.RoleServer {
		t.Fatalf("role mismatch: %q", marker.Role)
	}
	if marker.Reason == "" {
		t.Fatalf("expected non-empty reason")
	}

	gotXray, err := os.ReadFile(filepath.Join(liveConfigDir, layout.XrayConfigFileName))
	if err != nil {
		t.Fatalf("read live xray: %v", err)
	}
	if string(gotXray) != string(baselineXray) {
		t.Fatalf("expected live xray to be restored from lkg")
	}
}
