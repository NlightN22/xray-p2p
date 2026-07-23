//go:build windows || linux

package server

import (
	"path/filepath"
	"testing"
)

func TestSchemaCompatibilityV026ServerDesired(t *testing.T) {
	testSchemaCompatibilityServerDesired(t, "v0.2.6")
}

func TestSchemaCompatibilityV027ServerDesired(t *testing.T) {
	testSchemaCompatibilityServerDesired(t, "v0.2.7")
}

func testSchemaCompatibilityServerDesired(t *testing.T, release string) {
	t.Helper()
	path := filepath.Join("..", "..", "..", "tests", "schema", "compat", release, "xp2p-server.toml")
	doc, err := loadServerStateDoc(path)
	if err != nil {
		t.Fatalf("load %s server Desired: %v", release, err)
	}
	users, err := decodeServerTrojanUsers(doc)
	if err != nil {
		t.Fatalf("decode %s users: %v", release, err)
	}
	if len(users) == 0 {
		t.Fatalf("users = %#v", users)
	}
	if doc[serverRedirectRulesKey] == nil || doc[serverReverseStateKey] == nil || doc[serverForwardRulesKey] == nil {
		t.Fatalf("legacy server state was not preserved: %#v", doc)
	}
}
