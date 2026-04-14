//go:build linux || windows

package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestApplyPendingIfRequested_SkipsWhenPreviousFailureMatchesRequestID(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(root, "logs"))
	if err := os.MkdirAll(config.StateRoot(), 0o755); err != nil {
		t.Fatalf("mkdir state: %v", err)
	}

	desiredConfigDir := filepath.Join(root, "config-server")
	pendingConfigDir, err := config.PendingConfigDir(desiredConfigDir)
	if err != nil {
		t.Fatalf("PendingConfigDir: %v", err)
	}
	liveConfigDir, err := config.LiveConfigDir(desiredConfigDir)
	if err != nil {
		t.Fatalf("LiveConfigDir: %v", err)
	}

	req := apply.Request{ID: "req-1", Role: apply.RoleServer}
	if err := apply.WriteRequest(config.ApplyRequestPath(), req, ""); err != nil {
		t.Fatalf("WriteRequest: %v", err)
	}
	if err := apply.WriteError(config.ApplyErrorPath(), apply.ErrorMarker{
		RequestID: "req-1",
		Role:      apply.RoleServer,
		Reason:    "failed",
	}, ""); err != nil {
		t.Fatalf("WriteError: %v", err)
	}

	if err := os.MkdirAll(pendingConfigDir, 0o755); err != nil {
		t.Fatalf("mkdir pending: %v", err)
	}
	if err := os.MkdirAll(liveConfigDir, 0o755); err != nil {
		t.Fatalf("mkdir live: %v", err)
	}

	pendingConfig := config.PendingConfigPath(layout.ServerConfigFileName)
	if err := os.WriteFile(pendingConfig, []byte("pending\n"), 0o644); err != nil {
		t.Fatalf("write pending: %v", err)
	}

	rollback, applied, gotReq, err := applyPendingIfRequested(apply.RoleServer, desiredConfigDir)
	if err != nil {
		t.Fatalf("applyPendingIfRequested: %v", err)
	}
	if rollback != nil {
		t.Fatalf("expected rollback=nil on skip")
	}
	if applied {
		t.Fatalf("expected applied=false on skip")
	}
	if gotReq.ID != "req-1" {
		t.Fatalf("request id mismatch: %q", gotReq.ID)
	}

	liveConfig := config.LiveConfigPath(layout.ServerConfigFileName)
	if _, err := os.Stat(liveConfig); err == nil {
		t.Fatalf("expected live config to be untouched")
	}
	if _, err := os.Stat(pendingConfig); err != nil {
		t.Fatalf("expected pending config to remain: %v", err)
	}
}
