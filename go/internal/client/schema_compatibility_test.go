package client

import (
	"path/filepath"
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
}
