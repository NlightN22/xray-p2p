package client

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSchemaCompatibilityV026ClientDesired(t *testing.T) {
	path := filepath.Join("..", "..", "..", "tests", "schema", "compat", "v0.2.6", "xp2p-client.toml")
	state, err := loadClientInstallState(path)
	if err != nil {
		t.Fatalf("load v0.2.6 client Desired: %v", err)
	}
	if len(state.Endpoints) != 1 || state.Endpoints[0].Tag != "proxy-primary" {
		t.Fatalf("endpoints = %#v", state.Endpoints)
	}
	if len(state.Redirects) != 1 || len(state.Reverse) != 1 || len(state.Forwards) != 1 {
		t.Fatalf("legacy state was not preserved: %#v", state)
	}
	if state.Subscriptions == nil || len(state.Subscriptions) != 0 {
		t.Fatalf("legacy subscriptions were not normalized to an empty model: %#v", state.Subscriptions)
	}
	normalizedPath := filepath.Join(t.TempDir(), "xp2p-client.toml")
	if err := state.save(normalizedPath); err != nil {
		t.Fatalf("save normalized v0.2.6 client Desired: %v", err)
	}
	normalized, err := os.ReadFile(normalizedPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(normalized), "subscriptions") {
		t.Fatalf("empty external subscription model changed legacy Desired:\n%s", normalized)
	}
}
