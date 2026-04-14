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
	pendingConfigDir, err := config.PendingConfigDir(desiredConfigDir)
	if err != nil {
		t.Fatalf("PendingConfigDir: %v", err)
	}

	lkgRoot := config.LkgRoot()
	if err := os.MkdirAll(filepath.Join(lkgRoot, configDirName), 0o755); err != nil {
		t.Fatalf("mkdir lkg: %v", err)
	}
	lkgConfig := config.LkgConfigPath(layout.ServerConfigFileName)
	lkgConfigData := []byte("[server]\n")
	if err := os.WriteFile(lkgConfig, lkgConfigData, 0o644); err != nil {
		t.Fatalf("write lkg config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(lkgRoot, configDirName, "inbounds.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write lkg inbounds: %v", err)
	}

	appliedStatePath := config.ConfigPath(layout.ServerAppliedStateFileName)
	if err := os.MkdirAll(filepath.Dir(appliedStatePath), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.WriteFile(appliedStatePath, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write applied state: %v", err)
	}

	if err := os.MkdirAll(pendingConfigDir, 0o755); err != nil {
		t.Fatalf("mkdir pending dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pendingConfigDir, "inbounds.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write pending inbounds: %v", err)
	}
	pendingConfig := config.PendingConfigPath(layout.ServerConfigFileName)
	if err := os.WriteFile(pendingConfig, []byte("this is not toml\n"), 0o644); err != nil {
		t.Fatalf("write pending config: %v", err)
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

	liveConfig := config.LiveConfigPath(layout.ServerConfigFileName)
	gotLive, err := os.ReadFile(liveConfig)
	if err != nil {
		t.Fatalf("read live config: %v", err)
	}
	if string(gotLive) != string(lkgConfigData) {
		t.Fatalf("expected live config to be restored from lkg")
	}

	if _, err := os.Stat(pendingConfig); err == nil {
		t.Fatalf("expected pending config to be cleaned up")
	}
	if _, err := os.Stat(pendingConfigDir); err == nil {
		t.Fatalf("expected pending config dir to be cleaned up")
	}
}
