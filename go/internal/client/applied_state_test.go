package client

import (
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/forward"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

func TestClientAppliedStateSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "client.state.json")

	cfg := clientInstallState{
		Endpoints: []clientEndpointRecord{
			{
				Hostname:      "edge.example",
				Tag:           "proxy-edge",
				Address:       "edge.example",
				Port:          443,
				User:          "alpha@example.com",
				Password:      "secret",
				ServerName:    "edge.example",
				AllowInsecure: true,
			},
		},
		Redirects: []redirect.Rule{
			{Domain: "svc.example", OutboundTag: "proxy-edge"},
		},
		Forwards: []forward.Rule{
			{ListenAddress: "127.0.0.1", ListenPort: 10001, TargetHost: "192.0.2.10", TargetPort: 8080},
		},
	}

	if err := saveClientAppliedState(path, cfg, true, "xp2pc", 1500, "198.18.0.1/30"); err != nil {
		t.Fatalf("saveClientAppliedState failed: %v", err)
	}

	loaded, err := loadClientAppliedState(path)
	if err != nil {
		t.Fatalf("loadClientAppliedState failed: %v", err)
	}
	if !loaded.TunEnabled || loaded.TunName != "xp2pc" || loaded.TunMTU != 1500 {
		t.Fatalf("unexpected tun settings: %+v", loaded)
	}
	if loaded.Mode != "tun" {
		t.Fatalf("unexpected mode: %s", loaded.Mode)
	}
	if loaded.Version == "" {
		t.Fatalf("expected version to be set")
	}
	if loaded.Timestamp.IsZero() {
		t.Fatalf("expected timestamp to be set")
	}
	if len(loaded.Config.Endpoints) != 1 || loaded.Config.Endpoints[0].Hostname != "edge.example" {
		t.Fatalf("unexpected config endpoints: %+v", loaded.Config.Endpoints)
	}
}
