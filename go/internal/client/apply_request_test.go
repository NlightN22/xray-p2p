//go:build linux || windows

package client

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

	req := apply.Request{ID: "req-1", Role: apply.RoleClient}
	if err := apply.WriteRequest(config.ApplyRequestPath(), req, ""); err != nil {
		t.Fatalf("WriteRequest: %v", err)
	}
	if err := apply.WriteError(config.ApplyErrorPath(), apply.ErrorMarker{
		RequestID: "req-1",
		Role:      apply.RoleClient,
		Reason:    "failed",
	}, ""); err != nil {
		t.Fatalf("WriteError: %v", err)
	}

	rollback, applied, gotReq, err := applyPendingIfRequested(apply.RoleClient)
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

	liveDir, err := config.LiveRoleDir(apply.RoleClient)
	if err != nil {
		t.Fatalf("LiveRoleDir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(liveDir, layout.XrayConfigFileName)); err == nil {
		t.Fatalf("expected live artifacts to be untouched")
	}
}
