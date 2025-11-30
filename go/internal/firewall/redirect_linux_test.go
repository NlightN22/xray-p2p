//go:build linux

package firewall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanAddProducesSnippetAndEntryPath(t *testing.T) {
	dir := t.TempDir()
	manager := NewManager(filepath.Join(dir, "xray.nft"), filepath.Join(dir, "entries"))

	plan, err := manager.PlanAdd("10.0.0.0/24", 12345)
	if err != nil {
		t.Fatalf("PlanAdd returned error: %v", err)
	}
	if plan.SnippetPath != filepath.Join(dir, "xray.nft") {
		t.Fatalf("unexpected snippet path: %s", plan.SnippetPath)
	}
	if plan.EntryPath == "" || !strings.Contains(plan.EntryPath, "xray_redirect_10_0_0_0_24") {
		t.Fatalf("unexpected entry path: %s", plan.EntryPath)
	}
	if plan.Entry == nil || plan.Entry.Port != 12345 || plan.Entry.Subnet != "10.0.0.0/24" {
		t.Fatalf("plan entry not populated: %+v", plan.Entry)
	}
	if plan.Snippet == "" || !strings.Contains(plan.Snippet, "10.0.0.0/24") {
		t.Fatalf("snippet missing expected subnet:\n%s", plan.Snippet)
	}
	if len(plan.IPTables) == 0 {
		t.Fatalf("expected iptables commands")
	}
}

func TestPlanRemoveRespectsExistingEntry(t *testing.T) {
	dir := t.TempDir()
	entryDir := filepath.Join(dir, "entries")
	if err := os.MkdirAll(entryDir, 0o755); err != nil {
		t.Fatalf("failed to create entries dir: %v", err)
	}
	entryPath := filepath.Join(entryDir, "xray_redirect_10_0_0_0_24.entry")
	if err := os.WriteFile(entryPath, []byte("SUBNET=\"10.0.0.0/24\"\nPORT=\"12345\"\n"), 0o644); err != nil {
		t.Fatalf("failed to seed entry file: %v", err)
	}

	manager := NewManager(filepath.Join(dir, "xray.nft"), entryDir)
	plan, err := manager.PlanRemove("10.0.0.0/24", false)
	if err != nil {
		t.Fatalf("PlanRemove returned error: %v", err)
	}
	if plan.Snippet != "" {
		t.Fatalf("expected empty snippet after removal, got: %s", plan.Snippet)
	}
	if plan.RemoveAll {
		t.Fatalf("expected RemoveAll=false for single subnet")
	}
	if len(plan.IPTables) == 0 {
		t.Fatalf("expected iptables cleanup commands")
	}
}

func TestDetectDokodemoPorts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "inbounds.json")
	content := `{
		"inbounds": [
			{"protocol": "dokodemo-door", "port": 6000},
			{"protocol": "socks", "port": 1080},
			{"protocol": "dokodemo-door", "port": 7000}
		]
	}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write inbound file: %v", err)
	}

	ports, err := DetectDokodemoPorts(path)
	if err != nil {
		t.Fatalf("DetectDokodemoPorts returned error: %v", err)
	}
	if len(ports) != 2 || ports[0] != 6000 || ports[1] != 7000 {
		t.Fatalf("unexpected ports: %v", ports)
	}
}
