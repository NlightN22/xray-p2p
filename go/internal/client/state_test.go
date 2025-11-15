package client

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadClientInstallStateMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	state, err := loadClientInstallState(path)
	if err != nil {
		t.Fatalf("loadClientInstallState returned error: %v", err)
	}
	if len(state.Endpoints) != 0 || len(state.Redirects) != 0 || len(state.Reverse) != 0 {
		t.Fatalf("expected empty state, got %+v", state)
	}
}

func TestClientInstallStateSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	original := clientInstallState{
		Endpoints: []clientEndpointRecord{
			{Hostname: "alpha.example", Tag: "proxy-alpha"},
		},
		Redirects: []clientRedirectRule{
			{Domain: "svc.example", OutboundTag: "proxy-alpha"},
		},
		Reverse: map[string]clientReverseChannel{
			"alphaedge-example.rev": {UserID: "alpha", Host: "edge.example", Tag: "alphaedge-example.rev", EndpointTag: "proxy-alpha"},
		},
	}
	if err := original.save(path); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := loadClientInstallState(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(loaded.Endpoints) != 1 || loaded.Endpoints[0].Hostname != "alpha.example" {
		t.Fatalf("unexpected endpoints: %+v", loaded.Endpoints)
	}
	if len(loaded.Redirects) != 1 || loaded.Redirects[0].Domain != "svc.example" {
		t.Fatalf("unexpected redirects: %+v", loaded.Redirects)
	}
	if len(loaded.Reverse) != 1 || loaded.Reverse["alphaedge-example.rev"].EndpointTag != "proxy-alpha" {
		t.Fatalf("unexpected reverse map: %+v", loaded.Reverse)
	}
}

func TestClientInstallStateEnsureReverseChannel(t *testing.T) {
	state := clientInstallState{}
	channel, err := state.ensureReverseChannel("User.One", "edge.example", "proxy-alpha")
	if err != nil {
		t.Fatalf("ensureReverseChannel failed: %v", err)
	}
	if channel.Tag != "user-oneedge-example.rev" {
		t.Fatalf("unexpected tag: %s", channel.Tag)
	}

	if _, err := state.ensureReverseChannel("User.One", "edge.example", "proxy-alpha"); err != nil {
		t.Fatalf("expected repeated call to succeed, got %v", err)
	}

	if _, err := state.ensureReverseChannel("User.One", "edge.example", "proxy-beta"); err == nil {
		t.Fatalf("expected conflict when user mapped to a different endpoint tag")
	}

	state.removeReverseChannelsByTag("proxy-alpha")
	if len(state.Reverse) != 0 {
		t.Fatalf("expected reverse map to be empty, got %+v", state.Reverse)
	}
}

func TestClientInstallStateAddRedirectValidatesUniqueness(t *testing.T) {
	state := clientInstallState{}
	err := state.addRedirect(clientRedirectRule{Domain: "svc.example", OutboundTag: "proxy-alpha"})
	if err != nil {
		t.Fatalf("addRedirect failed: %v", err)
	}
	err = state.addRedirect(clientRedirectRule{Domain: "svc.example", OutboundTag: "proxy-alpha"})
	if err == nil {
		t.Fatalf("expected duplicate redirect to fail")
	}
}

func TestClientInstallStateRemoveEndpointAlsoRemovesRedirects(t *testing.T) {
	state := clientInstallState{
		Endpoints: []clientEndpointRecord{
			{Hostname: "alpha.example", Tag: "proxy-alpha"},
		},
		Redirects: []clientRedirectRule{
			{Domain: "svc.example", OutboundTag: "proxy-alpha"},
			{Domain: "beta.example", OutboundTag: "proxy-beta"},
		},
	}
	state.removeRedirectsByTag("proxy-alpha")
	if len(state.Redirects) != 1 || state.Redirects[0].OutboundTag != "proxy-beta" {
		t.Fatalf("expected only one redirect to remain, got %+v", state.Redirects)
	}
}

func TestLoadClientInstallStateIgnoresEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("   "), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	state, err := loadClientInstallState(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(state.Endpoints) != 0 {
		t.Fatalf("expected empty endpoints, got %+v", state.Endpoints)
	}
}
