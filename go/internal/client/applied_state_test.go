package client

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/forward"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
	"github.com/NlightN22/xray-p2p/go/internal/xrayguard"
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

func TestUpdateClientRuntimeQuarantine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "client.state.json")
	cfg := clientInstallState{
		Endpoints: []clientEndpointRecord{
			{Hostname: "edge.example", Tag: "proxy-edge"},
		},
	}
	if err := saveClientAppliedState(path, cfg, true, "xp2pc", 1500, "198.18.0.1/30"); err != nil {
		t.Fatalf("saveClientAppliedState failed: %v", err)
	}

	start := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)
	event := xrayguard.Event{
		Reason: xrayguard.ReasonFDSpike,
		PID:    77,
		Before: xrayguard.Sample{
			Timestamp:           start,
			FDCount:             22,
			SocketFDCount:       16,
			EstablishedTCPCount: 50,
		},
		After: xrayguard.Sample{
			Timestamp:           start.Add(2 * time.Second),
			FDCount:             4095,
			SocketFDCount:       4088,
			EstablishedTCPCount: 60,
		},
		Window:              2 * time.Second,
		FDDelta:             4073,
		SocketRatioPercent:  99,
		EstablishedTCPCount: 60,
		Action:              "kill_xray",
	}

	if err := updateClientRuntimeQuarantine(path, event, "proxy-edge"); err != nil {
		t.Fatalf("updateClientRuntimeQuarantine failed: %v", err)
	}

	loaded, err := loadClientAppliedState(path)
	if err != nil {
		t.Fatalf("loadClientAppliedState failed: %v", err)
	}
	if loaded.Runtime.Status != "quarantined" || loaded.Runtime.Reason != xrayguard.ReasonFDSpike {
		t.Fatalf("unexpected runtime status: %+v", loaded.Runtime)
	}
	loop := loaded.Runtime.LoopProtection
	if loop == nil {
		t.Fatal("expected loop protection state")
	}
	if loop.FDBefore != 22 || loop.FDAfter != 4095 || loop.RelatedOutbound != "proxy-edge" {
		t.Fatalf("unexpected loop protection state: %+v", loop)
	}
}
