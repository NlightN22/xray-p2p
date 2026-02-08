package server

import (
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/forward"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

func TestServerAppliedStateSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.state.json")

	reverse := serverReverseState{
		"alphaedge-example.rev": {
			UserID: "alpha",
			Host:   "edge.example",
			Tag:    "alphaedge-example.rev",
			Domain: "alphaedge-example.rev",
		},
	}
	redirects := []redirect.Rule{
		{Domain: "svc.example", OutboundTag: "alphaedge-example.rev"},
	}
	forwards := []forward.Rule{
		{ListenAddress: "127.0.0.1", ListenPort: 11001, TargetHost: "192.0.2.20", TargetPort: 8443},
	}

	if err := saveServerAppliedState(path, reverse, redirects, forwards, false, "xp2ps", 1400, "198.18.0.5/30"); err != nil {
		t.Fatalf("saveServerAppliedState failed: %v", err)
	}

	loaded, err := loadServerAppliedState(path)
	if err != nil {
		t.Fatalf("loadServerAppliedState failed: %v", err)
	}
	if loaded.TunEnabled {
		t.Fatalf("expected TunEnabled=false")
	}
	if loaded.Mode != "proxy" {
		t.Fatalf("unexpected mode: %s", loaded.Mode)
	}
	if loaded.Version == "" {
		t.Fatalf("expected version to be set")
	}
	if loaded.Timestamp.IsZero() {
		t.Fatalf("expected timestamp to be set")
	}
	if len(loaded.Reverse) != 1 {
		t.Fatalf("unexpected reverse state: %+v", loaded.Reverse)
	}
	if len(loaded.Redirects) != 1 || loaded.Redirects[0].Domain != "svc.example" {
		t.Fatalf("unexpected redirects: %+v", loaded.Redirects)
	}
	if len(loaded.Forwards) != 1 || loaded.Forwards[0].ListenPort != 11001 {
		t.Fatalf("unexpected forwards: %+v", loaded.Forwards)
	}
}
