package client

import (
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestListEndpoints(t *testing.T) {

	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
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

	configPath := filepath.Clean(config.PendingConfigPath(layout.ClientConfigFileName))
	initial := clientInstallState{
		Endpoints: []clientEndpointRecord{
			{
				Hostname:             "server-a.example",
				Tag:                  "proxy-server-a",
				Address:              "198.51.100.10",
				Port:                 8443,
				User:                 "alice@example.com",
				Password:             "alice-secret",
				ServerName:           "server-a.example",
				ALPN:                 []string{"h2", "http/1.1"},
				AllowInsecure:        false,
				PinnedPeerCertSHA256: "abc123",
				VerifyPeerCertByName: "server-a.example",
			},
			{
				Hostname:      "server-b.example",
				Tag:           "proxy-server-b",
				Address:       "203.0.113.20",
				Port:          9443,
				User:          "bob@example.com",
				Password:      "bob-secret",
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
		Pending:    true,
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
	if records[0].TLSMode != TLSModePinnedName {
		t.Fatalf("unexpected TLS mode for first endpoint: %s", records[0].TLSMode)
	}
	if records[1].TLSMode != TLSModeInsecure {
		t.Fatalf("unexpected TLS mode for second endpoint: %s", records[1].TLSMode)
	}
	if records[0].Link != "trojan://alice-secret@198.51.100.10:8443?alpn=h2%2Chttp%2F1.1&pinnedPeerCertSha256=abc123&security=tls&sni=server-a.example&verifyPeerCertByName=server-a.example#alice%2540example.com" {
		t.Fatalf("unexpected first link: %s", records[0].Link)
	}
	if records[1].Link != "trojan://bob-secret@203.0.113.20:9443?allowInsecure=1&security=tls&sni=server-b.example#bob%2540example.com" {
		t.Fatalf("unexpected second link: %s", records[1].Link)
	}
}
