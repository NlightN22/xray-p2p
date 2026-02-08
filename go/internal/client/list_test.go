package client

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestListEndpoints(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})

	configPath := filepath.Join(dir, layout.ClientConfigFileName)
	initial := clientInstallState{
		Endpoints: []clientEndpointRecord{
			{
				Hostname:      "server-a.example",
				Tag:           "proxy-server-a",
				Address:       "198.51.100.10",
				Port:          8443,
				User:          "alice@example.com",
				ServerName:    "server-a.example",
				AllowInsecure: false,
			},
			{
				Hostname:      "server-b.example",
				Tag:           "proxy-server-b",
				Address:       "203.0.113.20",
				Port:          9443,
				User:          "bob@example.com",
				ServerName:    "server-b.example",
				AllowInsecure: true,
			},
		},
	}
	if err := initial.save(configPath); err != nil {
		t.Fatalf("save config: %v", err)
	}

	records, err := ListEndpoints(ListOptions{
		InstallDir: dir,
		ConfigDir:  layout.ClientConfigDir,
	})
	if err != nil {
		t.Fatalf("ListEndpoints failed: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].Hostname != "server-a.example" || records[1].Tag != "proxy-server-b" {
		t.Fatalf("unexpected records: %+v", records)
	}
}
