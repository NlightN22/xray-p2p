package client

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

func TestListRedirectsReportsMixedRecords(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	configDirName := layout.ClientConfigDir
	if err := os.MkdirAll(filepath.Join(dir, configDirName), 0o755); err != nil {
		t.Fatalf("mkdir extensions dir: %v", err)
	}

	statePath := filepath.Clean(config.ConfigPath(layout.ClientConfigFileName))
	state := clientInstallState{
		Endpoints: []clientEndpointRecord{
			{Hostname: "server-a.example", Tag: "proxy-server-a"},
			{Hostname: "server-b.example", Tag: "proxy-server-b"},
		},
		Redirects: []redirect.Rule{
			{CIDR: "10.100.0.0/16", OutboundTag: "proxy-server-a"},
			{Domain: "svc.example.net", OutboundTag: "proxy-server-b"},
		},
	}
	if err := state.save(statePath); err != nil {
		t.Fatalf("save state: %v", err)
	}

	list, err := ListRedirects(RedirectListOptions{
		InstallDir: dir,
		ConfigDir:  configDirName,
		Pending:    true,
	})
	if err != nil {
		t.Fatalf("list redirects: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(list))
	}
	if list[0].Type != "CIDR" || list[0].Value != "10.100.0.0/16" || list[0].Hostname != "server-a.example" {
		t.Fatalf("unexpected first entry %+v", list[0])
	}
	if list[1].Type != "domain" || list[1].Value != "svc.example.net" || list[1].Hostname != "server-b.example" {
		t.Fatalf("unexpected second entry %+v", list[1])
	}
}
