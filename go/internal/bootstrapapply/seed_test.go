package bootstrapapply

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/apply"
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/version"
)

func TestSeedReplacesFailedRequestFromOlderCompiler(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	desired := filepath.Join(root, layout.ClientConfigFileName)
	if err := os.WriteFile(desired, []byte("[client]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	liveDir, err := config.LiveRoleDir(apply.RoleClient)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(liveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := []byte(`{"role":"client","version":"0.2.6"}`)
	if err := os.WriteFile(filepath.Join(liveDir, layout.RuntimeMetaFileName), meta, 0o644); err != nil {
		t.Fatal(err)
	}
	old := apply.Request{ID: "old-request", Role: apply.RoleClient}
	if err := apply.WriteRequest(config.ApplyRequestPath(), old, ""); err != nil {
		t.Fatal(err)
	}
	if err := apply.WriteError(config.ApplyErrorPath(), apply.ErrorMarker{
		RequestID: old.ID,
		Role:      old.Role,
		Reason:    "device or resource busy",
	}, ""); err != nil {
		t.Fatal(err)
	}

	seeded, err := Seed(apply.RoleClient, liveDir, filepath.Join(root, layout.ClientConfigDir))
	if err != nil {
		t.Fatal(err)
	}
	if !seeded {
		t.Fatal("expected a fresh apply generation")
	}
	req, exists, err := apply.ReadRequestForRole(config.ApplyRequestPath(), apply.RoleClient)
	if err != nil || !exists {
		t.Fatalf("read request: exists=%v err=%v", exists, err)
	}
	if req.ID == old.ID {
		t.Fatal("old failed request was retained")
	}
	if _, exists, err := apply.ReadError(config.ApplyErrorPath()); err != nil || exists {
		t.Fatalf("stale apply error remains: exists=%v err=%v", exists, err)
	}
}

func TestSeedKeepsFailedRequestFromCurrentCompiler(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", root)
	if err := os.WriteFile(filepath.Join(root, layout.ClientConfigFileName), []byte("[client]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	liveDir, err := config.LiveRoleDir(apply.RoleClient)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(liveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := []byte(`{"role":"client","version":"` + version.Current() + `"}`)
	if err := os.WriteFile(filepath.Join(liveDir, layout.RuntimeMetaFileName), meta, 0o644); err != nil {
		t.Fatal(err)
	}
	old := apply.Request{ID: "current-request", Role: apply.RoleClient}
	if err := apply.WriteRequest(config.ApplyRequestPath(), old, ""); err != nil {
		t.Fatal(err)
	}
	if err := apply.WriteError(config.ApplyErrorPath(), apply.ErrorMarker{
		RequestID: old.ID, Role: old.Role, Reason: "current failure",
	}, ""); err != nil {
		t.Fatal(err)
	}

	seeded, err := Seed(apply.RoleClient, liveDir, filepath.Join(root, layout.ClientConfigDir))
	if err != nil {
		t.Fatal(err)
	}
	if seeded {
		t.Fatal("current compiler request must remain unchanged")
	}
	req, exists, err := apply.ReadRequestForRole(config.ApplyRequestPath(), apply.RoleClient)
	if err != nil || !exists || req.ID != old.ID {
		t.Fatalf("current failed request changed: request=%+v exists=%v err=%v", req, exists, err)
	}
}
